class UI {
    constructor() {
        this.dropZone = document.getElementById('dropZone');
        this.fileInput = document.getElementById('fileInput');
        this.currentImage = null;
        this.imageGrid = document.getElementById('imageGrid');
        this.modal = document.getElementById('alertModal');
        this.initEventListeners();
        this.loadImages();
    }

    initEventListeners() {
        this.dropZone.addEventListener('click', () => this.fileInput.click());
        this.dropZone.addEventListener('dragover', e => this.handleDragOver(e));
        this.dropZone.addEventListener('drop', e => this.handleDrop(e));
        this.fileInput.addEventListener('change', () => this.handleFileSelect());
    }

    async handleFileSelect() {
        const files = Array.from(this.fileInput.files);
        if (files.length === 0) return;
        await this.processFiles(files);
    }

    async processFiles(files) {
        // Filter out non-image files
        const imageFiles = files.filter(file => file.type.startsWith('image/'));
        
        if (imageFiles.length === 0) {
            this.showAlert('No image files found in selection', true);
            return;
        }

        this.showProgress(true);
        try {
            const results = await Promise.allSettled(
                files.map(file => this.uploadFile(file))
            );

            const successes = results.filter(r => r.status === 'fulfilled');
            const errors = results.filter(r => r.status === 'rejected');

            if (successes.length > 0) {
                this.showAlert(`${successes.length} files uploaded successfully`, false);
                this.loadImages();
            }

            if (errors.length > 0) {
                const errorMessages = errors.map(e => e.reason.message);
                this.showAlert(
                    `${errors.length} files failed: ${errorMessages.join(', ')}`,
                    true
                );
            }
        } catch (error) {
            this.showAlert(`Upload failed: ${error.message}`, true);
        } finally {
            this.showProgress(false);
        }
    }

    async uploadFile(file) {
        const formData = new FormData();
        formData.append('image', file);

        const response = await fetch('/upload', {
            method: 'POST',
            body: formData
        });

        if (!response.ok) {
            const error = await response.json();
            throw new Error(error.message || 'Upload failed');
        }

        return await response.json();
    }

    async loadImages() {
        try {
            // First load current image
            const currentResponse = await fetch('/current-image');
            const currentData = await currentResponse.json();
            this.currentImage = currentData.data?.current || null;

            // Then load image list
            const listResponse = await fetch('/images');
            if (!listResponse.ok) throw new Error('Failed to load images');
            
            const { data: filenames } = await listResponse.json();
            this.renderImageGrid(filenames);
        } catch (error) {
            this.showAlert(error.message, true);
        }
    }

    renderImageGrid(filenames) {
        this.imageGrid.innerHTML = filenames.length === 0
            ? '<div class="empty-message">No images uploaded yet</div>'
            : filenames.map(filename => `
                <div class="image-card">
                    <img src="uploads/${filename}" alt="${filename}">
                    <div class="image-info">
                        <button class="select-button ${this.currentImage === filename ? 'selected' : ''}" 
                                data-filename="${filename}"
                                onclick="ui.handleSelectImage('${filename}')">
                            ${this.currentImage === filename ? 'Selected' : 'Select'}
                        </button>
                    </div>
                </div>
            `).join('');
    }

    async handleSelectImage(filename) {
        try {
            const formData = new FormData();
            formData.append('filename', filename);

            const response = await fetch('/select-image', {
                method: 'POST',
                body: formData
            });

            if (!response.ok) {
                const error = await response.json();
                throw new Error(error.message);
            }

            this.currentImage = filename;
            this.highlightSelectedImage(filename);
            this.showAlert('Image selected for streaming', false);
        } catch (error) {
            this.showAlert(error.message, true);
        }
    }

    highlightSelectedImage(filename) {
        document.querySelectorAll('.select-button').forEach(btn => {
            btn.textContent = btn.dataset.filename === filename ? 'Selected' : 'Select';
            btn.classList.toggle('selected', btn.dataset.filename === filename);
        });
    }

    showAlert(message, isError) {
        const messageEl = document.getElementById('modalMessage');
        messageEl.textContent = message;
        
        // Get CSS custom properties
        const rootStyles = getComputedStyle(document.documentElement);
        const errorColor = rootStyles.getPropertyValue('--error').trim();
        const successColor = rootStyles.getPropertyValue('--success').trim();
        
        messageEl.style.color = isError ? errorColor : successColor;
        this.modal.style.display = 'block';
    }

    handleDragOver(e) {
        e.preventDefault();
        this.dropZone.classList.add('dragover');
    }

    handleDrop(e) {
        e.preventDefault();
        this.dropZone.classList.remove('dragover');
        
        const items = e.dataTransfer.items;
        const files = [];
        
        // Handle directory drops
        const processEntry = (entry) => {
            return new Promise((resolve) => {
                if (entry.isFile) {
                    entry.file(file => {
                        files.push(file);
                        resolve();
                    });
                } else if (entry.isDirectory) {
                    const reader = entry.createReader();
                    reader.readEntries(entries => {
                        Promise.all(entries.map(processEntry)).then(resolve);
                    });
                }
            });
        };
    
        for (let i = 0; i < items.length; i++) {
            const entry = items[i].webkitGetAsEntry();
            if (entry) {
                processEntry(entry);
            }
        }
    
        // Process files after 100ms to allow directory traversal
        setTimeout(() => this.processFiles(files), 100);
    }

    showProgress(show) {
        document.getElementById('progressBar').style.visibility = 
            show ? 'visible' : 'hidden';
    }
}

// Initialize application
const ui = new UI();

// Close modal handlers
document.querySelector('.close').addEventListener('click', () => {
    document.getElementById('alertModal').style.display = 'none';
});

window.addEventListener('click', (e) => {
    if (e.target === document.getElementById('alertModal')) {
        document.getElementById('alertModal').style.display = 'none';
    }
});