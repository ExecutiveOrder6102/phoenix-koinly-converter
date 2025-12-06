const go = new Go();
let wasmLoaded = false;

// Load WASM
WebAssembly.instantiateStreaming(fetch("main.wasm"), go.importObject).then((result) => {
    go.run(result.instance);
    wasmLoaded = true;
    console.log("WASM Loaded");
}).catch(err => {
    console.error("Failed to load WASM:", err);
    showStatus("Failed to load WASM core. Please ensure main.wasm is present.", "error");
});

const dropZone = document.getElementById('drop-zone');
const fileInput = document.getElementById('file-input');
const statusDiv = document.getElementById('status');
const resultArea = document.getElementById('result-area');
const downloadBtn = document.getElementById('download-btn');

dropZone.addEventListener('click', () => fileInput.click());

dropZone.addEventListener('dragover', (e) => {
    e.preventDefault();
    dropZone.classList.add('dragover');
});

dropZone.addEventListener('dragleave', () => {
    dropZone.classList.remove('dragover');
});

dropZone.addEventListener('drop', (e) => {
    e.preventDefault();
    dropZone.classList.remove('dragover');
    if (e.dataTransfer.files.length) {
        handleFile(e.dataTransfer.files[0]);
    }
});

fileInput.addEventListener('change', (e) => {
    if (e.target.files.length) {
        handleFile(e.target.files[0]);
    }
});

function handleFile(file) {
    if (!wasmLoaded) {
        showStatus("WASM not loaded yet. Please wait...", "error");
        return;
    }

    const reader = new FileReader();
    reader.onload = async (e) => {
        const content = e.target.result;
        try {
            const output = convertPhoenixToKoinly(content);

            if (output.startsWith("Error")) {
                showStatus(output, "error");
                resultArea.classList.add('hidden');
            } else {
                showStatus("Conversion successful!", "success");
                setupDownload(output);
            }
        } catch (err) {
            showStatus("Error during conversion: " + err, "error");
        }
    };
    reader.readAsText(file);
}

function showStatus(msg, type) {
    statusDiv.textContent = msg;
    statusDiv.className = 'status ' + type;
}

function setupDownload(csvContent) {
    resultArea.classList.remove('hidden');

    downloadBtn.onclick = () => {
        const blob = new Blob([csvContent], { type: 'text/csv' });
        const url = URL.createObjectURL(blob);
        const a = document.createElement('a');
        a.href = url;
        a.download = 'koinly.csv';
        document.body.appendChild(a);
        a.click();
        document.body.removeChild(a);
        URL.revokeObjectURL(url);
    };
}
