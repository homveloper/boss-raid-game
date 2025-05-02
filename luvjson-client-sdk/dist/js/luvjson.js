/**
 * LuvJSON Client SDK for JavaScript
 */

class LuvJsonClient {
  /**
   * Create a new LuvJSON client
   * @param {Object} options - Configuration options
   * @param {string} options.wasmUrl - URL to the WASM file
   */
  constructor(options = {}) {
    this.wasmUrl = options.wasmUrl || '/wasm/luvjson.wasm';
    this.ready = false;
    this.readyPromise = this._initialize();
  }

  /**
   * Initialize the WASM module
   * @private
   */
  async _initialize() {
    if (typeof window === 'undefined') {
      throw new Error('LuvJsonClient is only available in browser environments');
    }

    // Load the WASM module
    const go = new window.Go();
    const result = await WebAssembly.instantiateStreaming(
      fetch(this.wasmUrl),
      go.importObject
    );
    
    go.run(result.instance);
    this.ready = true;
  }

  /**
   * Ensure the client is ready
   * @private
   */
  async _ensureReady() {
    if (!this.ready) {
      await this.readyPromise;
    }
  }

  /**
   * Create a new document
   * @param {string} documentId - The document ID
   * @returns {Promise<Object>} - Result object
   */
  async createDocument(documentId) {
    await this._ensureReady();
    return window.createDocument(documentId);
  }

  /**
   * Get document content
   * @param {string} documentId - The document ID
   * @returns {Promise<Object>} - Result object with content
   */
  async getDocumentContent(documentId) {
    await this._ensureReady();
    const result = window.getDocumentContent(documentId);
    
    if (result.success) {
      result.content = JSON.parse(result.content);
    }
    
    return result;
  }

  /**
   * Create an operation
   * @param {string} type - Operation type (add, remove, replace)
   * @param {string} path - JSON path
   * @param {any} value - Value for the operation
   * @param {string} clientId - Client ID
   * @returns {Promise<Object>} - Result object with operation
   */
  async createOperation(type, path, value, clientId) {
    await this._ensureReady();
    const valueJson = JSON.stringify(value);
    const result = window.createOperation(type, path, valueJson, clientId);
    
    if (result.success) {
      result.operation = JSON.parse(result.operation);
    }
    
    return result;
  }

  /**
   * Create a patch from operations
   * @param {string} documentId - The document ID
   * @param {Array} operations - Array of operations
   * @param {string} clientId - Client ID
   * @returns {Promise<Object>} - Result object with patch
   */
  async createPatch(documentId, operations, clientId) {
    await this._ensureReady();
    const opsJson = JSON.stringify(operations);
    const result = window.createPatch(documentId, opsJson, clientId);
    
    if (result.success) {
      result.patch = JSON.parse(result.patch);
    }
    
    return result;
  }

  /**
   * Apply a patch to a document
   * @param {string} documentId - The document ID
   * @param {Object} patch - The patch to apply
   * @returns {Promise<Object>} - Result object
   */
  async applyPatch(documentId, patch) {
    await this._ensureReady();
    const patchJson = JSON.stringify(patch);
    return window.applyPatch(documentId, patchJson);
  }
}

// Export for browser and Node.js environments
if (typeof window !== 'undefined') {
  window.LuvJsonClient = LuvJsonClient;
}

if (typeof module !== 'undefined' && module.exports) {
  module.exports = LuvJsonClient;
}
