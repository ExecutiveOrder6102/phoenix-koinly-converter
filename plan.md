# PostgreSQL Auditing Tool for SOC2/SOX Compliance

## 1. Overview and Objectives

This project delivers a **PostgreSQL auditing tool** to support SOC2/SOX-style compliance by regularly documenting database users and roles.

**Key outcomes:**

- Generate an **immutable, timestamped PDF** report (optionally digitally signed).
- Report lists **every user/role** in the database (equivalent to `\du+` in `psql`).
- Designed to be **extensible** for future audits (e.g. `pgaudit` checks).
- Runs as a **Go service on Cloud Run**, triggered via **Cloud Workflows** (or similar).
- Connects to **multiple Cloud SQL for Postgres instances**, discovered via a **config list** or **GCP project/folder scan**.
- Stores outputs in **GCS**, with an option for **local save in dev**.

---

## 2. High-Level Requirements

### 2.1 Functional

1. **Immutable PDF Generation**
   - Generate a PDF report per run.
   - Include:
     - Generation date/time (UTC).
     - For each instance:
       - List of all roles/users.
       - Role attributes (superuser, login, etc.).
       - Role memberships ("member of").
   - Make PDF read-only and optionally **digitally signed**.

2. **User & Role Data (like `\du+`)**
   - For each **Postgres instance**:
     - Enumerate roles from `pg_catalog`:
       - `rolname`
       - `rolsuper`, `rolcreaterole`, `rolcreatedb`
       - `rolcanlogin`, `rolreplication`, `rolbypassrls`
       - Role memberships (`pg_auth_members`)
       - Optional comments/descriptions
   - Output in a human-readable tabular format.

3. **Trigger Mechanism**
   - Preferred: **Cloud Workflows → HTTP → Cloud Run**.
   - Must be easy to demonstrate to auditors:
     - “We click this button / run this Workflow.”
     - Can screenshot Workflow execution logs as evidence.

4. **Deployment / Runtime**
   - Implemented in **Go**.
   - Containerized and deployed to **Cloud Run**.
   - Stateless; all persistent artifacts are in **GCS**.

5. **Multi-DB / Multi-Project Support**
   - Support 2 discovery modes:
     - **Configured list** of instances.
     - **Auto-discovery** via:
       - List of **projects**, and/or
       - List of **folders** (recursively list projects).
   - For GCP:
     - Use **Cloud SQL Admin API** to list Cloud SQL instances.
     - Filter for **Postgres** instances.

6. **Secure Connectivity**
   - Prefer **Cloud SQL Go Connector** with **IAM authentication**.
   - Fallback to **username/password** where required.
   - Private connectivity (no public IPs if possible).

7. **Output Storage**
   - Upload PDF to **GCS bucket**.
   - Filename includes timestamp.
   - In **dev**, optionally save to local disk.

8. **Extensibility**
   - Code structured so that new **audit tasks** can be added:
     - e.g. “Check that `pgaudit` is enabled for all `@<domain>` roles.”
   - PDF generator supports adding new sections.

---

## 3. Architecture

### 3.1 Components

1. **Cloud Run Service (Go application)**
   - HTTP API (e.g. `POST /run-audit`).
   - Responsibilities:
     - Parse config / input payload.
     - Discover target Postgres instances.
     - Connect to each DB via Cloud SQL Connector (IAM or user/pass).
     - Execute queries to fetch role info.
     - Generate PDF.
     - Upload PDF to GCS.
     - Return status + optional GCS path.

2. **Trigger: Cloud Workflows**
   - Simple workflow that:
     - Accepts optional input (e.g. list of projects/folders).
     - Issues authenticated HTTP POST to Cloud Run.
     - Waits for response.
     - Logs result (for screenshots & audit trail).

3. **Google Cloud Services**
   - **Cloud SQL Admin API**
     - List instances within a project.
   - **Cloud Resource Manager API**
     - List projects inside folders (optional auto-discovery).
   - **Cloud Storage**
     - Store generated PDFs.
   - **Cloud Logging**
     - Central logs from Cloud Run and Workflows.
   - **(Optional) Secret Manager**
     - Store DB credentials for non-IAM connections.
   - **(Optional) Cloud KMS**
     - Store private keys for PDF signing.

### 3.2 Flow

1. Operator runs the **Workflow** (manual trigger for quarterly audits).
2. Workflow calls **Cloud Run** endpoint (HTTP POST, OIDC auth).
3. Cloud Run service:
   - Reads configuration (env or payload).
   - Discovers Postgres instances (admin API / static list).
   - For each instance:
     - Connects securely via Cloud SQL Connector.
     - Queries roles & memberships.
   - Builds in-memory model of results.
   - Generates PDF (with timestamp & per-instance sections).
   - (Optionally) digitally signs PDF.
   - Uploads PDF to GCS.
   - Returns `200 OK` + metadata to Workflow.
4. Workflow logs the outcome (visible in console).
5. Auditor:
   - Is provided with PDF from GCS.
   - Can be shown Workflow execution log screenshot as evidence.

---

## 4. Detailed Design

### 4.1 Configuration

**Inputs to the system:**

- **Static config (env or JSON):**
  - List of instances:
    ```json
    {
      "instances": [
        {
          "project": "my-project",
          "region": "europe-west1",
          "instance": "postgres-prod",
          "db": "postgres"
        }
      ]
    }
    ```
- **Dynamic discovery:**
  - List of **projects** to scan.
  - List of **folders** to scan (recursively list projects).
- **Auth mode per instance:**
  - `auth_mode: "iam"` or `"password"`.
  - For `"password"`, reference to Secret Manager secret.
- **Output:**
  - `GCS_BUCKET_NAME`
  - Optional `LOCAL_SAVE_DIR` in dev.
- **Signing (optional, later stage):**
  - `SIGN_PDF=true/false`
  - `SIGN_CERT_SECRET` / `KMS_KEY_NAME` etc.

Config loading happens at request start, so a redeploy isn’t strictly necessary when changing discovery targets.

---

### 4.2 Discovery Module

**Responsibilities:**

- Given input (config, projects, folders), output a list of **InstanceTargets**:

  ```go
  type InstanceTarget struct {
      ProjectID    string
      Region       string
      InstanceName string
      DBName       string // usually "postgres"
      AuthMode     string // "iam" or "password"
      SecretName   string // for password mode
  }
  ```

**Tasks:**

1. **Static list path:**
   - Parse JSON or env config directly into `[]InstanceTarget`.

2. **Project-based discovery:**
   - For each `ProjectID`:
     - Use Cloud SQL Admin API to `List` instances.
     - Filter instances where `databaseVersion` indicates Postgres.
     - Create `InstanceTarget` entries (db name defaults to `postgres`).

3. **Folder-based discovery (optional):**
   - Use Cloud Resource Manager:
     - List projects in each folder (recursive if needed).
     - For each discovered project, run project-based discovery as above.

4. **Merging & de-duplication:**
   - Merge results from static list and discovered instances.
   - De-duplicate by `(project, region, instance)`.

---

### 4.3 Database Connection & Authentication

**Goal:** Provide a simple function:

```go
func ConnectPostgres(ctx context.Context, target InstanceTarget) (*pgx.Conn, error)
```

**Implementation notes:**

- Use the **Cloud SQL Go Connector**:
  - Create a connector dialer with:
    - Instance connection name: `project:region:instance`.
  - Use PGX config:
    ```go
    config, _ := pgx.ParseConfig("")
    config.DialFunc = func(ctx context.Context, network, addr string) (net.Conn, error) {
        return dialer.Dial(ctx, INSTANCE_CONNECTION_NAME)
    }
    ```
  - For IAM auth:
    - Connection user often set to a mapped IAM user.
    - Password is empty; connector injects short-lived token.
  - For password auth:
    - Use Secret Manager to fetch password.
    - Configure `config.User` and `config.Password`.

- Permissions:
  - Cloud Run service account needs:
    - `roles/cloudsql.client` to connect.
    - `roles/secretmanager.secretAccessor` if using password mode.

- Connection lifecycle:
  - For each instance:
    - Connect.
    - Run queries.
    - Close connection.

- Error handling:
  - If connection fails:
    - Log error.
    - Add entry to report like: “Instance X – CONNECTION FAILED: <error>”.
    - Continue with other instances.

---

### 4.4 Role Retrieval

**Goal:** Equivalent of `\du+` via SQL.

**Suggested query:**

```sql
SELECT
  r.rolname,
  r.rolsuper,
  r.rolcreaterole,
  r.rolcreatedb,
  r.rolcanlogin,
  r.rolreplication,
  r.rolbypassrls,
  ARRAY(
    SELECT b.rolname
    FROM pg_catalog.pg_auth_members m
    JOIN pg_catalog.pg_roles b ON m.roleid = b.oid
    WHERE m.member = r.oid
  ) AS memberof
FROM pg_catalog.pg_roles r
ORDER BY r.rolname;
```

(Optional: join `pg_shdescription` or `pg_description` for comments.)

**Data model:**

```go
type RoleInfo struct {
    Name         string
    CanLogin     bool
    IsSuperuser  bool
    CreateRole   bool
    CreateDB     bool
    Replication  bool
    BypassRLS    bool
    MemberOf     []string
    Description  string
}

type InstanceRolesReport struct {
    ProjectID    string
    Region       string
    InstanceName string
    DBName       string
    Roles        []RoleInfo
    Error        string // set if instance failed
}
```

**Logic:**

- For each `InstanceTarget`:
  - Connect.
  - Run query.
  - Scan rows into `RoleInfo`.
  - Append to `InstanceRolesReport`.
- Aggregate all reports in a top-level struct for PDF generation.

---

### 4.5 PDF Generation

**Goal:** Single PDF with:

- Cover/title page.
- One section per instance:
  - Instance identifier.
  - Table of roles.

**Library (suggested):**

- `gofpdf` (simple, OSS, good enough for tables).
- Later, if needed, migrate to a library that supports PDF signing.

**Layout:**

1. **Cover Page / Header**
   - Title: “PostgreSQL Role Audit Report”.
   - Subheader: environment or org name.
   - Date/time of generation.
   - Optional: Environment/region info.

2. **Per-Instance Section**
   - Heading: `<project>/<region>/<instance> (db: <dbName>)`.
   - If `Error != ""`:
     - Show a highlighted note: “Instance unreachable: <Error>”.
   - Otherwise, a table with columns, e.g.:
     - Role
     - Can Login
     - Superuser
     - CreateRole
     - CreateDB
     - Replication
     - BypassRLS
     - Member Of
   - Use “Yes/No” or “✔/✖” for booleans (auditor-friendly).
   - Wrap / truncate long lists in “Member Of”.

3. **Footer**
   - Tool name + version (e.g. “Generated by pg-audit-tool v1.0.0”).
   - Optional SHA-256 hash displayed for internal reference.

**Implementation steps:**

- Implement `GeneratePDF(report *FullAuditReport) ([]byte, error)`:
  - Create PDF.
  - Write pages & tables.
  - Return bytes.

- In Cloud Run:
  - Write PDF bytes to `/tmp/report-<timestamp>.pdf` or stream directly to GCS.

---

### 4.6 PDF Signing (Optional / Later Stage)

**Objective:** Provide a cryptographic guarantee that:

- The PDF was generated by the tool.
- It has not been modified.

**Approach (future iteration):**

- Store signing certificate/private key in:
  - Secret Manager, or
  - Cloud KMS (asymmetric key).
- Use a PDF library that supports digital signatures.
- Steps:
  1. Generate unsigned PDF (as now).
  2. Apply signature:
     - Create signature field.
     - Sign using the private key.
  3. Output signed PDF bytes.

**UX for auditors:**

- In standard PDF viewers:
  - They see a “Signed by <Org Name>” indicator.
  - If the document is altered, the signature shows as invalid.

In the initial version, keep this **behind a feature flag** or documented as a future enhancement.

---

### 4.7 GCS Storage Integration

**Goal:** Store PDFs in a configured bucket, with deterministic naming.

**Implementation:**

- Use `cloud.google.com/go/storage`.
- Bucket name from env: `AUDIT_PDF_BUCKET`.
- Object name pattern:
  - For a single PDF covering all instances:
    - `postgres_roles_audit/<YYYY>/<MM>/<DD>/roles_audit_<TIMESTAMP>.pdf`
  - Example:
    - `postgres_roles_audit/2025/12/31/roles_audit_2025-12-31T23-59-59Z.pdf`

**Steps:**

1. Create GCS client with default credentials.
2. Upload PDF bytes.
3. Log full `gs://` path.
4. Return object name to caller (Workflow).

**Dev mode:**

- If `LOCAL_SAVE_DIR` is set:
  - Save `report.pdf` locally.
  - Optionally skip GCS upload.

---

### 4.8 Trigger Workflow

**Workflow definition (simplified YAML sketch):**

```yaml
main:
  params: [input]
  steps:
    callAudit:
      call: http.post
      args:
        url: ${sys.get_env("AUDIT_SERVICE_URL")}
        auth:
          type: OIDC
        body:
          # optionally pass projects/folders/config overrides
          config: ${input.config}
      result: auditResult

    checkStatus:
      switch:
        - condition: ${auditResult.body.status == "ok"}
          next: done
        - condition: ${true}
          next: fail

    done:
      return:
        message: "Audit completed successfully"
        gcsPath: ${auditResult.body.gcsPath}

    fail:
      raise: "Audit failed"
```

**Permissions:**

- Workflow SA:
  - `roles/run.invoker` on the Cloud Run service.
- Execution:
  - Operator navigates to Workflow in GCP Console, clicks “Run”, then “View Execution” for logs.

This provides a **clear, human-readable trace** to show auditors.

---

## 5. Extensibility for Future Audits

### 5.1 Audit Task Abstraction

Define an interface:

```go
type AuditTask interface {
    Name() string
    Run(ctx context.Context, targets []InstanceTarget) (*TaskResult, error)
}
```

For now, implement one task:

- `RolesAuditTask`:
  - Connects to DBs, fetches role info, populates `InstanceRolesReport`.

Future tasks could be:

- `PGAuditConfigTask`:
  - Checks `pgaudit` settings per role:
    - Query role-level GUCs or config tables.
    - Verify roles matching `*@domain.com` have required audit logging.
  - Outputs pass/fail and details.

The PDF generator can then iterate over task results and add **new sections** to the report.

### 5.2 Adding a `pgaudit` Check (Future Example)

- Query `pg_settings` or role-based `ALTER ROLE ... SET pgaudit.*` values.
- For each role matching a pattern:
  - Assert required `pgaudit` config is present.
- Render in PDF as:
  - “PGAudit Checks” section with:
    - Table of roles & expected vs actual settings.
    - Summary of passes/fails.

---

## 6. Implementation Roadmap (Parallelizable Tasks)

### Task A: Repo & Infrastructure Setup

- Create `go` module and basic project structure.
- Dockerfile for Cloud Run.
- Terraform or equivalent for:
  - Cloud Run service.
  - GCS bucket.
  - Service accounts & IAM roles.
  - Workflow skeleton.

### Task B: Discovery Module

- Implement config parsing.
- Implement project-based and folder-based discovery.
- Unit tests with mocked API clients.

### Task C: DB Connection Module

- Wrap Cloud SQL Go Connector usage.
- Implement IAM and password auth modes.
- Integrate Secret Manager for passwords.
- Basic connection tests.

### Task D: Role Retrieval & Data Modeling

- Implement SQL queries.
- Map rows into `RoleInfo` / `InstanceRolesReport`.
- Handle errors per-instance.

### Task E: PDF Generation

- Integrate `gofpdf` (or chosen library).
- Implement report layout.
- Test with small and large datasets.

### Task F: GCS Integration

- Upload PDF to bucket.
- Return `gs://` path.
- Tests with fake or test buckets.

### Task G: Cloud Workflow & Trigger

- Define workflow YAML.
- Bind service accounts & IAM roles.
- Manual test execution path.

### Task H: Hardening & Docs

- Add logging & error handling.
- Add basic metrics (optional).
- Write runbook:
  - How to trigger.
  - Where to find PDFs.
  - How to present to auditors.

### Task I: Optional PDF Signing (Later)

- Research/choose signing library.
- Integrate with KMS/Secret Manager.
- Toggle via config flag.
- Document verification steps for auditors.

---

## 7. Deliverables

- **Go service** source code (Cloud Run ready).
- **Dockerfile** and deployment scripts.
- **Cloud Workflow** definition.
- **Terraform/Infra-as-code** for:
  - Cloud Run.
  - Workflow.
  - GCS bucket.
  - IAM bindings.
- **Runbook / README** explaining:
  - Configuration.
  - Triggering process.
  - How to consume the PDF.
  - Future extension points.
