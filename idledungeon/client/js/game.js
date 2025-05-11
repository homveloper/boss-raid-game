/**
 * IdleDungeon Game Renderer
 * Handles rendering the game state on a canvas
 */
class GameRenderer {
  /**
   * Create a new game renderer
   * @param {HTMLCanvasElement} canvas - Canvas element
   */
  constructor(canvas) {
    this.canvas = canvas;
    this.ctx = canvas.getContext('2d');
    this.gameState = null;
    this.playerUnit = null;
    this.selectedUnit = null;
    this.mouseX = 0;
    this.mouseY = 0;
    this.scale = 1;
    this.offsetX = 0;
    this.offsetY = 0;
    this.dragging = false;
    this.lastX = 0;
    this.lastY = 0;

    // Set up event listeners
    this._setupEventListeners();

    // Start animation loop
    this._animate();
  }

  /**
   * Set up event listeners
   * @private
   */
  _setupEventListeners() {
    // Mouse move
    this.canvas.addEventListener('mousemove', (e) => {
      const rect = this.canvas.getBoundingClientRect();
      this.mouseX = (e.clientX - rect.left) / this.scale + this.offsetX;
      this.mouseY = (e.clientY - rect.top) / this.scale + this.offsetY;

      if (this.dragging) {
        const dx = (e.clientX - rect.left) - this.lastX;
        const dy = (e.clientY - rect.top) - this.lastY;
        this.offsetX -= dx / this.scale;
        this.offsetY -= dy / this.scale;
        this.lastX = e.clientX - rect.left;
        this.lastY = e.clientY - rect.top;
      }
    });

    // Mouse down
    this.canvas.addEventListener('mousedown', (e) => {
      const rect = this.canvas.getBoundingClientRect();
      this.lastX = e.clientX - rect.left;
      this.lastY = e.clientY - rect.top;
      this.dragging = true;
      this.canvas.style.cursor = 'grabbing';

      // Check if clicked on a unit
      if (this.gameState) {
        const clickX = this.mouseX;
        const clickY = this.mouseY;
        
        this.selectedUnit = null;
        for (const unitId in this.gameState.units) {
          const unit = this.gameState.units[unitId];
          const dx = unit.x - clickX;
          const dy = unit.y - clickY;
          const distance = Math.sqrt(dx * dx + dy * dy);
          
          if (distance < 20) {
            this.selectedUnit = unit;
            break;
          }
        }
      }
    });

    // Mouse up
    this.canvas.addEventListener('mouseup', () => {
      this.dragging = false;
      this.canvas.style.cursor = 'default';
    });

    // Mouse leave
    this.canvas.addEventListener('mouseleave', () => {
      this.dragging = false;
      this.canvas.style.cursor = 'default';
    });

    // Mouse wheel (zoom)
    this.canvas.addEventListener('wheel', (e) => {
      e.preventDefault();
      const zoom = e.deltaY < 0 ? 1.1 : 0.9;
      this.scale *= zoom;
      
      // Limit zoom
      if (this.scale < 0.5) this.scale = 0.5;
      if (this.scale > 2) this.scale = 2;
      
      // Adjust offset to zoom toward mouse position
      this.offsetX += (this.mouseX - this.offsetX) * (1 - zoom);
      this.offsetY += (this.mouseY - this.offsetY) * (1 - zoom);
    });
  }

  /**
   * Animation loop
   * @private
   */
  _animate() {
    this._render();
    requestAnimationFrame(() => this._animate());
  }

  /**
   * Render the game state
   * @private
   */
  _render() {
    const { width, height } = this.canvas;
    
    // Clear canvas
    this.ctx.clearRect(0, 0, width, height);
    
    // Set transform
    this.ctx.save();
    this.ctx.translate(-this.offsetX * this.scale, -this.offsetY * this.scale);
    this.ctx.scale(this.scale, this.scale);
    
    // Draw grid
    this._drawGrid();
    
    // Draw units
    if (this.gameState) {
      for (const unitId in this.gameState.units) {
        const unit = this.gameState.units[unitId];
        this._drawUnit(unit);
      }
    }
    
    // Restore transform
    this.ctx.restore();
    
    // Draw UI
    this._drawUI();
  }

  /**
   * Draw the grid
   * @private
   */
  _drawGrid() {
    const { width, height } = this.canvas;
    const gridSize = 50;
    
    // Calculate grid bounds
    const startX = Math.floor(this.offsetX / gridSize) * gridSize;
    const startY = Math.floor(this.offsetY / gridSize) * gridSize;
    const endX = startX + width / this.scale + gridSize;
    const endY = startY + height / this.scale + gridSize;
    
    // Draw grid lines
    this.ctx.strokeStyle = 'rgba(200, 200, 200, 0.3)';
    this.ctx.lineWidth = 1;
    
    // Vertical lines
    for (let x = startX; x <= endX; x += gridSize) {
      this.ctx.beginPath();
      this.ctx.moveTo(x, startY);
      this.ctx.lineTo(x, endY);
      this.ctx.stroke();
    }
    
    // Horizontal lines
    for (let y = startY; y <= endY; y += gridSize) {
      this.ctx.beginPath();
      this.ctx.moveTo(startX, y);
      this.ctx.lineTo(endX, y);
      this.ctx.stroke();
    }
  }

  /**
   * Draw a unit
   * @param {Object} unit - Unit to draw
   * @private
   */
  _drawUnit(unit) {
    if (!unit.isAlive) return;
    
    const isPlayer = unit.type === 'player';
    const isSelected = this.selectedUnit && this.selectedUnit.id === unit.id;
    const isCurrentPlayer = this.playerUnit && this.playerUnit.id === unit.id;
    
    // Draw unit circle
    this.ctx.beginPath();
    this.ctx.arc(unit.x, unit.y, 15, 0, Math.PI * 2);
    
    // Fill based on unit type
    if (isPlayer) {
      this.ctx.fillStyle = isCurrentPlayer ? '#3498db' : '#2ecc71';
    } else {
      this.ctx.fillStyle = '#e74c3c';
    }
    
    this.ctx.fill();
    
    // Draw selection indicator
    if (isSelected) {
      this.ctx.strokeStyle = '#f39c12';
      this.ctx.lineWidth = 3;
      this.ctx.stroke();
    }
    
    // Draw health bar
    const healthPercent = unit.health / unit.maxHealth;
    const barWidth = 30;
    const barHeight = 5;
    
    this.ctx.fillStyle = '#e74c3c';
    this.ctx.fillRect(unit.x - barWidth / 2, unit.y - 25, barWidth, barHeight);
    
    this.ctx.fillStyle = '#2ecc71';
    this.ctx.fillRect(unit.x - barWidth / 2, unit.y - 25, barWidth * healthPercent, barHeight);
    
    // Draw name
    this.ctx.fillStyle = '#fff';
    this.ctx.font = '12px Arial';
    this.ctx.textAlign = 'center';
    this.ctx.fillText(unit.name, unit.x, unit.y - 30);
  }

  /**
   * Draw the UI
   * @private
   */
  _drawUI() {
    // Draw selected unit info
    if (this.selectedUnit) {
      const { width } = this.canvas;
      const padding = 10;
      const boxWidth = 200;
      const boxHeight = 100;
      const x = width - boxWidth - padding;
      const y = padding;
      
      // Draw info box
      this.ctx.fillStyle = 'rgba(44, 62, 80, 0.8)';
      this.ctx.fillRect(x, y, boxWidth, boxHeight);
      
      // Draw unit info
      this.ctx.fillStyle = '#fff';
      this.ctx.font = '14px Arial';
      this.ctx.textAlign = 'left';
      this.ctx.fillText(`Name: ${this.selectedUnit.name}`, x + 10, y + 20);
      this.ctx.fillText(`Type: ${this.selectedUnit.type}`, x + 10, y + 40);
      this.ctx.fillText(`Health: ${this.selectedUnit.health}/${this.selectedUnit.maxHealth}`, x + 10, y + 60);
      this.ctx.fillText(`Position: (${Math.round(this.selectedUnit.x)}, ${Math.round(this.selectedUnit.y)})`, x + 10, y + 80);
    }
  }

  /**
   * Update the game state
   * @param {Object} gameState - New game state
   */
  updateGameState(gameState) {
    this.gameState = gameState;
    
    // Update player unit
    if (window.app && window.app.playerId && gameState) {
      this.playerUnit = gameState.units[window.app.playerId] || null;
      
      // Center on player if first update
      if (this.playerUnit && !this._initialCentered) {
        this.offsetX = this.playerUnit.x - this.canvas.width / (2 * this.scale);
        this.offsetY = this.playerUnit.y - this.canvas.height / (2 * this.scale);
        this._initialCentered = true;
      }
    }
  }
}
