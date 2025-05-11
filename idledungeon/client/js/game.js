/**
 * GameRenderer handles rendering the game on a canvas
 */
class GameRenderer {
    /**
     * Create a new GameRenderer
     * @param {HTMLCanvasElement} canvas - Canvas element to render on
     */
    constructor(canvas) {
        this.canvas = canvas;
        this.ctx = canvas.getContext('2d');
        this.game = null;
        this.player = null;
        
        // Camera settings
        this.cameraX = 0;
        this.cameraY = 0;
        this.zoom = 1;
        this.isDragging = false;
        this.lastMouseX = 0;
        this.lastMouseY = 0;
        
        // Unit settings
        this.unitRadius = 20;
        this.playerColor = '#4caf50';
        this.monsterColor = '#f44336';
        this.selectedUnitColor = '#ffeb3b';
        this.selectedUnit = null;
        
        // Animation settings
        this.animationFrame = null;
        
        // Resize canvas on window resize
        window.addEventListener('resize', () => this.resizeCanvas());
        this.resizeCanvas();
        
        // Add event listeners for camera control
        this.setupEventListeners();
    }
    
    /**
     * Resize canvas to fit container
     */
    resizeCanvas() {
        const container = this.canvas.parentElement;
        this.canvas.width = container.clientWidth;
        this.canvas.height = container.clientHeight;
        this.render();
    }
    
    /**
     * Set up event listeners for camera control
     */
    setupEventListeners() {
        // Mouse down event
        this.canvas.addEventListener('mousedown', (event) => {
            this.isDragging = true;
            this.lastMouseX = event.clientX;
            this.lastMouseY = event.clientY;
            
            // Check if a unit was clicked
            const worldPos = this.screenToWorld(event.clientX, event.clientY);
            this.handleUnitClick(worldPos.x, worldPos.y);
        });
        
        // Mouse move event
        this.canvas.addEventListener('mousemove', (event) => {
            if (this.isDragging) {
                const dx = event.clientX - this.lastMouseX;
                const dy = event.clientY - this.lastMouseY;
                
                this.cameraX -= dx / this.zoom;
                this.cameraY -= dy / this.zoom;
                
                this.lastMouseX = event.clientX;
                this.lastMouseY = event.clientY;
                
                this.render();
            }
        });
        
        // Mouse up event
        this.canvas.addEventListener('mouseup', () => {
            this.isDragging = false;
        });
        
        // Mouse leave event
        this.canvas.addEventListener('mouseleave', () => {
            this.isDragging = false;
        });
        
        // Mouse wheel event for zooming
        this.canvas.addEventListener('wheel', (event) => {
            event.preventDefault();
            
            // Get mouse position before zoom
            const mouseX = event.clientX;
            const mouseY = event.clientY;
            const worldPos = this.screenToWorld(mouseX, mouseY);
            
            // Adjust zoom level
            const zoomFactor = event.deltaY > 0 ? 0.9 : 1.1;
            this.zoom *= zoomFactor;
            
            // Limit zoom level
            this.zoom = Math.max(0.1, Math.min(5, this.zoom));
            
            // Adjust camera to keep mouse position fixed
            const newWorldPos = this.screenToWorld(mouseX, mouseY);
            this.cameraX += worldPos.x - newWorldPos.x;
            this.cameraY += worldPos.y - newWorldPos.y;
            
            this.render();
        });
        
        // Double click event for moving player
        this.canvas.addEventListener('dblclick', (event) => {
            if (this.player) {
                const worldPos = this.screenToWorld(event.clientX, event.clientY);
                this.onPlayerMove(worldPos.x, worldPos.y);
            }
        });
    }
    
    /**
     * Convert screen coordinates to world coordinates
     * @param {number} screenX - Screen X coordinate
     * @param {number} screenY - Screen Y coordinate
     * @returns {Object} World coordinates {x, y}
     */
    screenToWorld(screenX, screenY) {
        const canvasRect = this.canvas.getBoundingClientRect();
        const canvasX = screenX - canvasRect.left;
        const canvasY = screenY - canvasRect.top;
        
        const worldX = this.cameraX + canvasX / this.zoom;
        const worldY = this.cameraY + canvasY / this.zoom;
        
        return { x: worldX, y: worldY };
    }
    
    /**
     * Convert world coordinates to screen coordinates
     * @param {number} worldX - World X coordinate
     * @param {number} worldY - World Y coordinate
     * @returns {Object} Screen coordinates {x, y}
     */
    worldToScreen(worldX, worldY) {
        const screenX = (worldX - this.cameraX) * this.zoom;
        const screenY = (worldY - this.cameraY) * this.zoom;
        
        return { x: screenX, y: screenY };
    }
    
    /**
     * Handle unit click
     * @param {number} worldX - World X coordinate
     * @param {number} worldY - World Y coordinate
     */
    handleUnitClick(worldX, worldY) {
        if (!this.game) return;
        
        // Check if a player was clicked
        for (const playerId in this.game.players) {
            const player = this.game.players[playerId];
            const dx = player.x - worldX;
            const dy = player.y - worldY;
            const distance = Math.sqrt(dx * dx + dy * dy);
            
            if (distance <= this.unitRadius) {
                this.selectedUnit = player;
                this.render();
                return;
            }
        }
        
        // Check if a monster was clicked
        for (const monsterId in this.game.monsters) {
            const monster = this.game.monsters[monsterId];
            const dx = monster.x - worldX;
            const dy = monster.y - worldY;
            const distance = Math.sqrt(dx * dx + dy * dy);
            
            if (distance <= this.unitRadius) {
                this.selectedUnit = monster;
                this.render();
                
                // If player is selected and monster is clicked, attack
                if (this.player && this.player.id !== this.selectedUnit.id) {
                    this.onMonsterAttack(this.selectedUnit.id);
                }
                
                return;
            }
        }
        
        // No unit clicked, deselect
        this.selectedUnit = null;
        this.render();
    }
    
    /**
     * Set the game state
     * @param {Object} game - Game state
     */
    setGame(game) {
        this.game = game;
        this.render();
    }
    
    /**
     * Set the player
     * @param {Object} player - Player object
     */
    setPlayer(player) {
        this.player = player;
        
        // Center camera on player
        if (player) {
            this.cameraX = player.x - this.canvas.width / (2 * this.zoom);
            this.cameraY = player.y - this.canvas.height / (2 * this.zoom);
        }
        
        this.render();
    }
    
    /**
     * Start rendering loop
     */
    start() {
        if (this.animationFrame) {
            cancelAnimationFrame(this.animationFrame);
        }
        
        const animate = () => {
            this.render();
            this.animationFrame = requestAnimationFrame(animate);
        };
        
        this.animationFrame = requestAnimationFrame(animate);
    }
    
    /**
     * Stop rendering loop
     */
    stop() {
        if (this.animationFrame) {
            cancelAnimationFrame(this.animationFrame);
            this.animationFrame = null;
        }
    }
    
    /**
     * Render the game
     */
    render() {
        if (!this.ctx) return;
        
        // Clear canvas
        this.ctx.clearRect(0, 0, this.canvas.width, this.canvas.height);
        
        if (!this.game) return;
        
        // Draw world grid
        this.drawGrid();
        
        // Draw monsters
        for (const monsterId in this.game.monsters) {
            const monster = this.game.monsters[monsterId];
            this.drawUnit(monster, this.monsterColor);
        }
        
        // Draw players
        for (const playerId in this.game.players) {
            const player = this.game.players[playerId];
            this.drawUnit(player, this.playerColor);
        }
        
        // Draw selected unit highlight
        if (this.selectedUnit) {
            const screenPos = this.worldToScreen(this.selectedUnit.x, this.selectedUnit.y);
            this.ctx.beginPath();
            this.ctx.arc(screenPos.x, screenPos.y, this.unitRadius * this.zoom * 1.2, 0, Math.PI * 2);
            this.ctx.strokeStyle = this.selectedUnitColor;
            this.ctx.lineWidth = 2;
            this.ctx.stroke();
            
            // Draw selected unit info
            this.drawUnitInfo(this.selectedUnit, screenPos.x, screenPos.y - this.unitRadius * this.zoom * 2);
        }
    }
    
    /**
     * Draw a grid
     */
    drawGrid() {
        const gridSize = 100;
        const gridColor = 'rgba(255, 255, 255, 0.1)';
        
        // Calculate grid boundaries
        const startX = Math.floor(this.cameraX / gridSize) * gridSize;
        const startY = Math.floor(this.cameraY / gridSize) * gridSize;
        const endX = this.cameraX + this.canvas.width / this.zoom;
        const endY = this.cameraY + this.canvas.height / this.zoom;
        
        this.ctx.strokeStyle = gridColor;
        this.ctx.lineWidth = 1;
        
        // Draw vertical lines
        for (let x = startX; x <= endX; x += gridSize) {
            const screenX = (x - this.cameraX) * this.zoom;
            this.ctx.beginPath();
            this.ctx.moveTo(screenX, 0);
            this.ctx.lineTo(screenX, this.canvas.height);
            this.ctx.stroke();
        }
        
        // Draw horizontal lines
        for (let y = startY; y <= endY; y += gridSize) {
            const screenY = (y - this.cameraY) * this.zoom;
            this.ctx.beginPath();
            this.ctx.moveTo(0, screenY);
            this.ctx.lineTo(this.canvas.width, screenY);
            this.ctx.stroke();
        }
    }
    
    /**
     * Draw a unit
     * @param {Object} unit - Unit to draw
     * @param {string} color - Unit color
     */
    drawUnit(unit, color) {
        const screenPos = this.worldToScreen(unit.x, unit.y);
        
        // Draw unit circle
        this.ctx.beginPath();
        this.ctx.arc(screenPos.x, screenPos.y, this.unitRadius * this.zoom, 0, Math.PI * 2);
        this.ctx.fillStyle = color;
        this.ctx.fill();
        
        // Draw health bar
        const healthBarWidth = this.unitRadius * 2 * this.zoom;
        const healthBarHeight = 5 * this.zoom;
        const healthBarX = screenPos.x - healthBarWidth / 2;
        const healthBarY = screenPos.y + (this.unitRadius + 5) * this.zoom;
        
        // Background
        this.ctx.fillStyle = 'rgba(0, 0, 0, 0.5)';
        this.ctx.fillRect(healthBarX, healthBarY, healthBarWidth, healthBarHeight);
        
        // Health
        const healthPercent = unit.health / unit.maxHealth;
        this.ctx.fillStyle = healthPercent > 0.5 ? '#4caf50' : healthPercent > 0.25 ? '#ff9800' : '#f44336';
        this.ctx.fillRect(healthBarX, healthBarY, healthBarWidth * healthPercent, healthBarHeight);
        
        // Draw name
        this.ctx.font = `${12 * this.zoom}px Arial`;
        this.ctx.fillStyle = 'white';
        this.ctx.textAlign = 'center';
        this.ctx.fillText(unit.name, screenPos.x, screenPos.y - (this.unitRadius + 5) * this.zoom);
    }
    
    /**
     * Draw unit info
     * @param {Object} unit - Unit to draw info for
     * @param {number} x - X coordinate
     * @param {number} y - Y coordinate
     */
    drawUnitInfo(unit, x, y) {
        const padding = 10 * this.zoom;
        const lineHeight = 20 * this.zoom;
        
        // Draw background
        this.ctx.fillStyle = 'rgba(0, 0, 0, 0.7)';
        this.ctx.fillRect(x - 100 * this.zoom, y - padding, 200 * this.zoom, 80 * this.zoom);
        
        // Draw info
        this.ctx.font = `${14 * this.zoom}px Arial`;
        this.ctx.fillStyle = 'white';
        this.ctx.textAlign = 'center';
        
        this.ctx.fillText(`${unit.name} (${unit.type})`, x, y + lineHeight);
        this.ctx.fillText(`Health: ${unit.health}/${unit.maxHealth}`, x, y + lineHeight * 2);
        this.ctx.fillText(`Attack: ${unit.attack} | Defense: ${unit.defense}`, x, y + lineHeight * 3);
    }
    
    /**
     * Set the player move callback
     * @param {Function} callback - Callback function
     */
    setPlayerMoveCallback(callback) {
        this.onPlayerMove = callback;
    }
    
    /**
     * Set the monster attack callback
     * @param {Function} callback - Callback function
     */
    setMonsterAttackCallback(callback) {
        this.onMonsterAttack = callback;
    }
    
    /**
     * Zoom in
     */
    zoomIn() {
        this.zoom *= 1.2;
        this.zoom = Math.min(5, this.zoom);
        this.render();
    }
    
    /**
     * Zoom out
     */
    zoomOut() {
        this.zoom *= 0.8;
        this.zoom = Math.max(0.1, this.zoom);
        this.render();
    }
}
