/**
 * Main application logic
 */
document.addEventListener('DOMContentLoaded', () => {
    // DOM elements
    const connectBtn = document.getElementById('connect-btn');
    const connectionStatus = document.getElementById('connection-status');
    const connectionPanel = document.getElementById('connection-panel');
    const gamePanel = document.getElementById('game-panel');
    const gameList = document.getElementById('game-list');
    const createGameBtn = document.getElementById('create-game-btn');
    const createGameForm = document.getElementById('create-game-form');
    const gameNameInput = document.getElementById('game-name-input');
    const submitGameBtn = document.getElementById('submit-game-btn');
    const cancelGameBtn = document.getElementById('cancel-game-btn');
    const joinGameContainer = document.getElementById('join-game-container');
    const gameListContainer = document.getElementById('game-list-container');
    const selectedGameInfo = document.getElementById('selected-game-info');
    const playerNameInput = document.getElementById('player-name-input');
    const joinGameBtn = document.getElementById('join-game-btn');
    const backToListBtn = document.getElementById('back-to-list-btn');
    const playerPanel = document.getElementById('player-panel');
    const playerInfo = document.getElementById('player-info');
    const playerStats = document.getElementById('player-stats');
    const gameViewContainer = document.getElementById('game-view-container');
    const gameCanvas = document.getElementById('game-canvas');
    const zoomInBtn = document.getElementById('zoom-in-btn');
    const zoomOutBtn = document.getElementById('zoom-out-btn');
    
    // Game state
    let syncClient = null;
    let gameRenderer = null;
    let selectedGame = null;
    let currentPlayer = null;
    let currentGameId = null;
    
    // Initialize game renderer
    gameRenderer = new GameRenderer(gameCanvas);
    
    // Set up event handlers for player movement and monster attacks
    gameRenderer.setPlayerMoveCallback((x, y) => {
        if (currentPlayer && currentGameId) {
            movePlayer(currentGameId, currentPlayer.id, x, y);
        }
    });
    
    gameRenderer.setMonsterAttackCallback((monsterId) => {
        if (currentPlayer && currentGameId) {
            attackMonster(currentGameId, currentPlayer.id, monsterId);
        }
    });
    
    // Connect button click handler
    connectBtn.addEventListener('click', () => {
        if (syncClient && syncClient.isConnected()) {
            syncClient.disconnect();
            connectBtn.textContent = '연결';
            connectionStatus.textContent = '연결 상태: 연결 안됨';
            connectionStatus.classList.remove('connected');
            connectionStatus.classList.add('disconnected');
            gamePanel.classList.add('hidden');
        } else {
            // Create sync client
            syncClient = new EventSyncClient({
                serverUrl: window.location.origin,
                onConnect: (data) => {
                    console.log('Connected to server', data);
                    connectBtn.textContent = '연결 해제';
                    connectionStatus.textContent = '연결 상태: 연결됨';
                    connectionStatus.classList.remove('disconnected');
                    connectionStatus.classList.add('connected');
                    gamePanel.classList.remove('hidden');
                    
                    // Load game list
                    loadGameList();
                },
                onDisconnect: () => {
                    console.log('Disconnected from server');
                    connectBtn.textContent = '연결';
                    connectionStatus.textContent = '연결 상태: 연결 안됨';
                    connectionStatus.classList.remove('connected');
                    connectionStatus.classList.add('disconnected');
                    gamePanel.classList.add('hidden');
                },
                onEvent: (event) => {
                    console.log('Received event', event);
                    
                    // Handle event based on type
                    if (event.operation === 'update' && event.documentId === currentGameId) {
                        // Reload game data
                        loadGame(currentGameId);
                    }
                },
                onError: (error) => {
                    console.error('Sync client error', error);
                    connectionStatus.textContent = '연결 상태: 오류 발생';
                    connectionStatus.classList.remove('connected');
                    connectionStatus.classList.add('disconnected');
                }
            });
            
            // Connect to server (without document ID for now)
            syncClient.connect('000000000000000000000000');
        }
    });
    
    // Create game button click handler
    createGameBtn.addEventListener('click', () => {
        createGameForm.classList.remove('hidden');
        gameNameInput.focus();
    });
    
    // Submit game button click handler
    submitGameBtn.addEventListener('click', () => {
        const gameName = gameNameInput.value.trim();
        if (gameName) {
            createGame(gameName);
            createGameForm.classList.add('hidden');
            gameNameInput.value = '';
        }
    });
    
    // Cancel game button click handler
    cancelGameBtn.addEventListener('click', () => {
        createGameForm.classList.add('hidden');
        gameNameInput.value = '';
    });
    
    // Back to list button click handler
    backToListBtn.addEventListener('click', () => {
        joinGameContainer.classList.add('hidden');
        gameListContainer.classList.remove('hidden');
        selectedGame = null;
    });
    
    // Join game button click handler
    joinGameBtn.addEventListener('click', () => {
        const playerName = playerNameInput.value.trim();
        if (playerName && selectedGame) {
            joinGame(selectedGame.id, playerName);
        }
    });
    
    // Zoom buttons click handlers
    zoomInBtn.addEventListener('click', () => {
        gameRenderer.zoomIn();
    });
    
    zoomOutBtn.addEventListener('click', () => {
        gameRenderer.zoomOut();
    });
    
    // Load game list
    function loadGameList() {
        fetch('/api/games')
            .then(response => response.json())
            .then(data => {
                if (data.success) {
                    // Clear game list
                    gameList.innerHTML = '';
                    
                    // Add games to list
                    const games = data.data;
                    if (games.length === 0) {
                        gameList.innerHTML = '<div class="no-games">게임이 없습니다. 새 게임을 생성하세요.</div>';
                    } else {
                        games.forEach(game => {
                            const gameItem = document.createElement('div');
                            gameItem.className = 'game-item';
                            gameItem.innerHTML = `
                                <div>${game.name}</div>
                                <div>플레이어: ${Object.keys(game.players).length}</div>
                                <div>상태: ${getGameStateText(game.state)}</div>
                            `;
                            
                            gameItem.addEventListener('click', () => {
                                // Select game
                                document.querySelectorAll('.game-item').forEach(item => {
                                    item.classList.remove('selected');
                                });
                                gameItem.classList.add('selected');
                                selectedGame = game;
                                
                                // Show join game form
                                gameListContainer.classList.add('hidden');
                                joinGameContainer.classList.remove('hidden');
                                
                                // Show selected game info
                                selectedGameInfo.innerHTML = `
                                    <div><strong>${game.name}</strong></div>
                                    <div>플레이어: ${Object.keys(game.players).length}</div>
                                    <div>상태: ${getGameStateText(game.state)}</div>
                                    <div>생성일: ${new Date(game.createdAt).toLocaleString()}</div>
                                `;
                                
                                playerNameInput.focus();
                            });
                            
                            gameList.appendChild(gameItem);
                        });
                    }
                } else {
                    console.error('Failed to load game list', data.error);
                }
            })
            .catch(error => {
                console.error('Failed to load game list', error);
            });
    }
    
    // Create a new game
    function createGame(name) {
        fetch('/api/games/create', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json'
            },
            body: JSON.stringify({ name })
        })
            .then(response => response.json())
            .then(data => {
                if (data.success) {
                    console.log('Game created', data.data);
                    loadGameList();
                } else {
                    console.error('Failed to create game', data.error);
                }
            })
            .catch(error => {
                console.error('Failed to create game', error);
            });
    }
    
    // Join a game
    function joinGame(gameId, playerName) {
        fetch('/api/games/join', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json'
            },
            body: JSON.stringify({ gameId, name: playerName })
        })
            .then(response => response.json())
            .then(data => {
                if (data.success) {
                    console.log('Joined game', data.data);
                    
                    // Set current game and player
                    currentGameId = data.data.game.id;
                    currentPlayer = data.data.player;
                    
                    // Update UI
                    joinGameContainer.classList.add('hidden');
                    playerPanel.classList.remove('hidden');
                    gameViewContainer.classList.remove('hidden');
                    
                    // Update player info
                    updatePlayerInfo(currentPlayer);
                    
                    // Set game in renderer
                    gameRenderer.setGame(data.data.game);
                    gameRenderer.setPlayer(currentPlayer);
                    gameRenderer.start();
                    
                    // Connect to game events
                    if (syncClient) {
                        syncClient.disconnect();
                        syncClient.connect(currentGameId);
                    }
                } else {
                    console.error('Failed to join game', data.error);
                }
            })
            .catch(error => {
                console.error('Failed to join game', error);
            });
    }
    
    // Load a game
    function loadGame(gameId) {
        fetch(`/api/games/get?id=${gameId}`)
            .then(response => response.json())
            .then(data => {
                if (data.success) {
                    console.log('Loaded game', data.data);
                    
                    // Update current player
                    if (currentPlayer && data.data.players[currentPlayer.id]) {
                        currentPlayer = data.data.players[currentPlayer.id];
                        updatePlayerInfo(currentPlayer);
                    }
                    
                    // Update game in renderer
                    gameRenderer.setGame(data.data);
                } else {
                    console.error('Failed to load game', data.error);
                }
            })
            .catch(error => {
                console.error('Failed to load game', error);
            });
    }
    
    // Move player
    function movePlayer(gameId, playerId, x, y) {
        fetch('/api/games/move', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json'
            },
            body: JSON.stringify({ gameId, playerId, x, y })
        })
            .then(response => response.json())
            .then(data => {
                if (data.success) {
                    console.log('Player moved', data.data);
                } else {
                    console.error('Failed to move player', data.error);
                }
            })
            .catch(error => {
                console.error('Failed to move player', error);
            });
    }
    
    // Attack monster
    function attackMonster(gameId, playerId, monsterId) {
        fetch('/api/games/attack', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json'
            },
            body: JSON.stringify({ gameId, playerId, monsterId })
        })
            .then(response => response.json())
            .then(data => {
                if (data.success) {
                    console.log('Monster attacked', data.data);
                } else {
                    console.error('Failed to attack monster', data.error);
                }
            })
            .catch(error => {
                console.error('Failed to attack monster', error);
            });
    }
    
    // Update player info
    function updatePlayerInfo(player) {
        playerInfo.innerHTML = `
            <div><strong>${player.name}</strong></div>
            <div>체력: ${player.health}/${player.maxHealth}</div>
            <div>공격력: ${player.attack}</div>
            <div>방어력: ${player.defense}</div>
            <div>골드: ${player.gold}</div>
        `;
        
        // Update health bar
        playerStats.innerHTML = `
            <div>체력:</div>
            <div class="stat-bar">
                <div class="stat-bar-fill health-bar-fill" style="width: ${(player.health / player.maxHealth) * 100}%"></div>
            </div>
            <div>골드:</div>
            <div class="stat-bar">
                <div class="stat-bar-fill gold-bar-fill" style="width: 100%"></div>
            </div>
        `;
    }
    
    // Get game state text
    function getGameStateText(state) {
        switch (state) {
            case 'waiting':
                return '대기 중';
            case 'playing':
                return '진행 중';
            case 'finished':
                return '종료됨';
            default:
                return state;
        }
    }
});
