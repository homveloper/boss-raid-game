// Global state
const state = {
    gameStatus: 'disconnected', // disconnected, connected, waiting, playing, finished
    selectedRoom: null,
    playerName: '',
    playerId: '',
    gameId: '',
    game: null,
    character: null,
    inventory: {},
    eventSource: null,
    attackInterval: null,
    lastAttackTime: 0,
    attackSpeed: 2000, // Default attack speed in milliseconds
    bossActionInterval: null,
    lastBossActionTime: 0,
    bossActionSpeed: 3000 // Default boss action request interval in milliseconds
};

// Game client object for external access
const gameClient = {
    get gameId() { return state.gameId; },
    get playerId() { return state.playerId; },
    get playerName() { return state.playerName; },
    get game() { return state.game; },

    // Initialize client with random user ID
    initialize() {
        // Generate random user ID if not already set
        if (!state.playerId) {
            state.playerId = 'user_' + Math.random().toString(36).substring(2, 15);
            state.playerName = 'Guest_' + Math.random().toString(36).substring(2, 7);
            console.log('Generated random user ID:', state.playerId);
            console.log('Generated random user name:', state.playerName);

            // Create a temporary game ID for crafting system
            if (!state.gameId) {
                state.gameId = 'game_' + Math.random().toString(36).substring(2, 15);
                console.log('Generated temporary game ID:', state.gameId);
            }

            // Save to localStorage for persistence
            localStorage.setItem('playerId', state.playerId);
            localStorage.setItem('playerName', state.playerName);
            localStorage.setItem('gameId', state.gameId);

            // Show notification
            this.showNotification(`환영합니다! 임시 사용자 ID가 생성되었습니다: ${state.playerName}`);
        }

        return {
            playerId: state.playerId,
            playerName: state.playerName,
            gameId: state.gameId
        };
    },

    // Get player name by ID
    getPlayerName(playerId) {
        if (playerId === state.playerId) {
            return state.playerName;
        }

        if (state.game && state.game.players && state.game.players[playerId]) {
            return state.game.players[playerId].name;
        }
        return playerId; // Return ID if name not found
    },

    // Show notification
    showNotification(message, type = 'info') {
        addEventToLog('system', message);
        // You could also implement a more sophisticated notification system here
    }
};

// Expose gameClient globally
window.gameClient = gameClient;

// Update game status
function updateGameStatus(status, message = '') {
    state.gameStatus = status;
    const statusText = document.getElementById('game-status-text');

    switch (status) {
        case 'disconnected':
            statusText.textContent = 'Not connected';
            statusText.className = 'status-disconnected';
            break;
        case 'connected':
            statusText.textContent = 'Connected';
            statusText.className = 'status-connected';
            break;
        case 'waiting':
            statusText.textContent = 'Waiting for players';
            statusText.className = 'status-waiting';
            break;
        case 'playing':
            statusText.textContent = 'In battle';
            statusText.className = 'status-playing';
            break;
        case 'finished':
            statusText.textContent = 'Game over';
            statusText.className = 'status-finished';
            break;
    }

    if (message) {
        addEventToLog('system', message);
    }

    updateUIBasedOnGameStatus();
}

// Update UI elements based on game status
function updateUIBasedOnGameStatus() {
    const readyBtn = document.getElementById('ready-btn');
    const attackBtn = document.getElementById('attack-btn');
    const leaveBtn = document.getElementById('leave-btn');
    const resultsPanel = document.getElementById('results-panel');

    // Reset all buttons
    readyBtn.disabled = true;
    attackBtn.disabled = true;
    leaveBtn.disabled = true;
    resultsPanel.classList.add('hidden');

    // Enable buttons based on game status
    switch (state.gameStatus) {
        case 'connected':
            leaveBtn.disabled = false;
            break;
        case 'waiting':
            readyBtn.disabled = false;
            leaveBtn.disabled = false;
            break;
        case 'playing':
            attackBtn.disabled = false;
            leaveBtn.disabled = false;
            break;
        case 'finished':
            leaveBtn.disabled = false;
            resultsPanel.classList.remove('hidden');
            break;
    }
}

// API functions
async function createRoom(name) {
    try {
        const response = await fetch('/api/rooms/create', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json'
            },
            body: JSON.stringify({ name })
        });

        if (!response.ok) {
            throw new Error('Failed to create room');
        }

        return await response.json();
    } catch (error) {
        console.error('Error creating room:', error);
        alert('Failed to create room. Please try again.');
        return null;
    }
}

async function getRooms() {
    try {
        const response = await fetch('/api/rooms');

        if (!response.ok) {
            throw new Error('Failed to get rooms');
        }

        return await response.json();
    } catch (error) {
        console.error('Error getting rooms:', error);
        return [];
    }
}

async function getRoom(id) {
    try {
        const response = await fetch(`/api/rooms/get?id=${id}`);

        if (!response.ok) {
            throw new Error('Failed to get room');
        }

        return await response.json();
    } catch (error) {
        console.error('Error getting room:', error);
        return null;
    }
}

async function joinRoom(roomId, playerName) {
    try {
        const response = await fetch('/api/rooms/join', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json'
            },
            body: JSON.stringify({ roomId, playerName })
        });

        if (!response.ok) {
            throw new Error('Failed to join game');
        }

        return await response.json();
    } catch (error) {
        console.error('Error joining game:', error);
        alert('Failed to join game. Please try again.');
        return null;
    }
}

async function readyGame(gameId, playerId) {
    try {
        const response = await fetch('/api/games/ready', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json'
            },
            body: JSON.stringify({ gameId, playerId })
        });

        if (!response.ok) {
            throw new Error('Failed to set ready status');
        }

        return await response.json();
    } catch (error) {
        console.error('Error setting ready status:', error);
        alert('Failed to set ready status. Please try again.');
        return null;
    }
}

async function attackBoss(gameId, playerId) {
    try {
        const response = await fetch('/api/games/attack', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json'
            },
            body: JSON.stringify({ gameId, playerId })
        });

        if (!response.ok) {
            throw new Error('Failed to attack boss');
        }

        return await response.json();
    } catch (error) {
        console.error('Error attacking boss:', error);
        alert('Failed to attack boss. Please try again.');
        return null;
    }
}

async function triggerBossAction(gameId, playerId) {
    try {
        const response = await fetch('/api/games/boss-action', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json'
            },
            body: JSON.stringify({ gameId, playerId })
        });

        if (!response.ok) {
            // Don't throw error for boss action - it's expected to fail sometimes
            // (e.g., when boss can't attack yet)
            console.log('Boss action not triggered:', await response.text());
            return null;
        }

        return await response.json();
    } catch (error) {
        console.error('Error triggering boss action:', error);
        return null;
    }
}

async function equipItem(gameId, playerId, itemId) {
    try {
        const response = await fetch('/api/games/equip', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json'
            },
            body: JSON.stringify({ gameId, playerId, itemId })
        });

        if (!response.ok) {
            throw new Error('Failed to equip item');
        }

        return await response.json();
    } catch (error) {
        console.error('Error equipping item:', error);
        alert('Failed to equip item. Please try again.');
        return null;
    }
}

async function makeMove(gameId, playerId, row, col) {
    try {
        const response = await fetch('/api/games/move', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json'
            },
            body: JSON.stringify({ gameId, playerId, row, col })
        });

        if (!response.ok) {
            throw new Error('Failed to make move');
        }

        return await response.json();
    } catch (error) {
        console.error('Error making move:', error);
        alert('Failed to make move. Please try again.');
        return null;
    }
}

async function getGame(id) {
    try {
        const response = await fetch(`/api/games/get?id=${id}`);

        if (!response.ok) {
            throw new Error('Failed to get game');
        }

        return await response.json();
    } catch (error) {
        console.error('Error getting game:', error);
        return null;
    }
}

// Event source functions
function connectToEventSource(gameId, playerId) {
    if (state.eventSource) {
        state.eventSource.close();
    }

    const url = `/api/events?gameId=${gameId}&playerId=${playerId}`;
    state.eventSource = new EventSource(url);

    state.eventSource.onmessage = (event) => {
        console.log('SSE event received:', event.data);
        try {
            const data = JSON.parse(event.data);
            handleEvent(data);
        } catch (error) {
            console.error('Error parsing SSE event:', error);
        }
    };

    state.eventSource.onerror = (error) => {
        console.error('EventSource error:', error);
        state.eventSource.close();
        setTimeout(() => {
            connectToEventSource(gameId, playerId);
        }, 1000);
    };

    console.log(`Connected to SSE for game ${gameId} as player ${playerId}`);
}

function disconnectFromEventSource() {
    if (state.eventSource) {
        state.eventSource.close();
        state.eventSource = null;
    }
    stopAutoAttack();
    stopBossActionTimer();
}

// Auto-attack functions
function startAutoAttack() {
    // Stop any existing auto-attack interval
    stopAutoAttack();

    // Set the attack speed based on the player's character if available
    if (state.game && state.game.players && state.game.players[state.playerId]) {
        const player = state.game.players[state.playerId];
        // Calculate attack speed based on character stats or equipment
        // For now, we'll use a default value that can be adjusted later
        state.attackSpeed = 2000; // 2 seconds between attacks

        // If the player has a weapon equipped, adjust attack speed
        if (player.character.equipment.weapon) {
            // Faster weapons reduce attack speed
            if (player.character.equipment.weapon.type === 'dagger') {
                state.attackSpeed = 1500; // 1.5 seconds
            } else if (player.character.equipment.weapon.type === 'bow') {
                state.attackSpeed = 2500; // 2.5 seconds
            } else if (player.character.equipment.weapon.type === 'longsword') {
                state.attackSpeed = 2000; // 2 seconds
            } else if (player.character.equipment.weapon.type === 'axe') {
                state.attackSpeed = 3000; // 3 seconds
            }
        }
    }

    console.log(`Starting auto-attack with speed: ${state.attackSpeed}ms`);

    // Start the auto-attack interval
    state.attackInterval = setInterval(async () => {
        // Only attack if the game is in playing state and player is alive
        if (state.game &&
            state.game.state === 'playing' &&
            state.game.players &&
            state.game.players[state.playerId] &&
            state.game.players[state.playerId].character.stats.health > 0) {

            // Check if enough time has passed since the last attack
            const now = Date.now();
            if (now - state.lastAttackTime >= state.attackSpeed) {
                state.lastAttackTime = now;

                // Visual feedback for auto-attack
                const attackBtn = document.getElementById('attack-btn');
                attackBtn.classList.add('attacking');
                setTimeout(() => {
                    attackBtn.classList.remove('attacking');
                }, 200);

                // Send the attack request
                await attackBoss(state.gameId, state.playerId);
            }
        }
    }, 500); // Check every 500ms if we can attack
}

function stopAutoAttack() {
    if (state.attackInterval) {
        clearInterval(state.attackInterval);
        state.attackInterval = null;
        console.log('Auto-attack stopped');
    }
}

// Boss action functions
function startBossActionTimer() {
    // Stop any existing boss action timer
    stopBossActionTimer();

    console.log(`Starting boss action timer with interval: ${state.bossActionSpeed}ms`);

    // Start the boss action timer
    state.bossActionInterval = setInterval(async () => {
        // Only trigger boss actions if the game is in playing state
        if (state.game &&
            state.game.state === 'playing') {

            // Check if enough time has passed since the last boss action request
            const now = Date.now();
            if (now - state.lastBossActionTime >= state.bossActionSpeed) {
                state.lastBossActionTime = now;

                // Send the boss action request
                await triggerBossAction(state.gameId, state.playerId);
            }
        }
    }, 1000); // Check every second if we can trigger a boss action
}

function stopBossActionTimer() {
    if (state.bossActionInterval) {
        clearInterval(state.bossActionInterval);
        state.bossActionInterval = null;
        console.log('Boss action timer stopped');
    }
}

// Event handlers
function handleEvent(event) {
    console.log('Received event:', event);

    // Dispatch custom event for other components to listen to
    const customEvent = new CustomEvent('game_event', { detail: event });
    document.dispatchEvent(customEvent);

    switch (event.type) {
        case 'game_state':
        case 'game_update':
            updateGameState(event.payload);
            break;
        case 'player_join':
            updateGameState(event.payload.game);
            addEventToLog('player_join', event.payload.description);
            break;
        case 'player_ready':
            updateGameState(event.payload.game);
            addEventToLog('player_ready', event.payload.description);
            break;
        case 'player_attack':
            updateGameState(event.payload.game);
            addEventToLog('player_attack', event.payload.description);
            break;
        case 'boss_attack':
            updateGameState(event.payload.game);
            addEventToLog('boss_attack', event.payload.description);
            break;
        case 'player_defeated':
            updateGameState(event.payload.game);
            addEventToLog('player_defeated', event.payload.description);
            break;
        case 'game_start':
            updateGameState(event.payload.game);
            addEventToLog('game_start', event.payload.description);
            updateGameStatus('playing', 'The battle has begun!');
            // Start auto-attacking and boss action timer when the game starts
            startAutoAttack();
            startBossActionTimer();
            break;
        case 'game_end':
            updateGameState(event.payload.game);
            addEventToLog('game_end', event.payload.description);
            updateGameStatus('finished');
            showGameResult(event.payload);
            // Stop auto-attacking and boss action timer when the game ends
            stopAutoAttack();
            stopBossActionTimer();
            break;
        case 'item_equip':
            updateGameState(event.payload.game);
            addEventToLog('item_equip', event.payload.description);
            break;
        // 제작 시스템 관련 이벤트
        case 'crafting_started':
            updateGameState(event.payload.game);
            addEventToLog('crafting', event.payload.description);
            break;
        case 'crafting_helped':
            updateGameState(event.payload.game);
            addEventToLog('crafting', event.payload.description);
            break;
        case 'crafting_completed':
            updateGameState(event.payload.game);
            addEventToLog('crafting', event.payload.description);
            break;
        case 'crafting_update':
            // 실시간 제작 상태 업데이트 이벤트
            addEventToLog('crafting', event.payload.description);
            // 커스텀 이벤트 발생시켜 제작 시스템에 알림
            const craftingUpdateEvent = new CustomEvent('crafting_update', {
                detail: event.payload.data
            });
            document.dispatchEvent(craftingUpdateEvent);
            break;
    }
}

// UI update functions
function updateGameState(game) {
    const oldGame = state.game;
    state.game = game;

    // Update game status based on game state
    if (game.state !== oldGame?.state) {
        if (game.state === 'waiting') {
            updateGameStatus('waiting');
        } else if (game.state === 'playing') {
            updateGameStatus('playing');
            startAutoAttack();
            startBossActionTimer();
        } else if (game.state === 'finished') {
            updateGameStatus('finished');
            stopAutoAttack();
            stopBossActionTimer();
        }
    }

    // If equipment changed, restart auto-attack to update attack speed
    if (game.state === 'playing' &&
        oldGame &&
        game.players &&
        oldGame.players &&
        game.players[state.playerId] &&
        oldGame.players[state.playerId] &&
        JSON.stringify(game.players[state.playerId].character.equipment) !==
        JSON.stringify(oldGame.players[state.playerId].character.equipment)) {
        startAutoAttack();
    }

    // Update room name
    const currentRoomName = document.getElementById('current-room-name');
    if (state.selectedRoom) {
        currentRoomName.textContent = state.selectedRoom.name;
    }

    // Update players list
    updatePlayersList();

    // Update character info
    updateCharacterInfo();

    // Update boss info
    updateBossInfo();
}

// Update players list
function updatePlayersList() {
    const game = state.game;
    if (!game || !game.players) return;

    const playersList = document.getElementById('players-list');
    playersList.innerHTML = '';

    Object.values(game.players).forEach(player => {
        const playerItem = document.createElement('div');
        playerItem.className = 'player-item';

        const playerName = document.createElement('span');
        playerName.textContent = player.name;

        const playerStatus = document.createElement('span');

        if (game.state === 'waiting') {
            playerStatus.textContent = player.ready ? 'Ready' : 'Not Ready';
            playerStatus.className = player.ready ? 'player-ready' : 'player-not-ready';
        } else if (game.state === 'playing' || game.state === 'finished') {
            if (player.character) {
                playerStatus.textContent = `HP: ${player.character.stats.health}`;
                if (player.character.stats.health <= 0) {
                    playerStatus.className = 'player-defeated';
                } else {
                    playerStatus.className = 'player-alive';
                }
            }
        }

        playerItem.appendChild(playerName);
        playerItem.appendChild(playerStatus);
        playersList.appendChild(playerItem);

        // Disable ready button if the player is already ready
        if (game.state === 'waiting' && player.id === state.playerId && player.ready) {
            document.getElementById('ready-btn').disabled = true;
        }
    });
}

// Update character info
function updateCharacterInfo() {
    const game = state.game;
    if (!game || !game.players || !game.players[state.playerId]) return;

    const player = game.players[state.playerId];
    if (!player.character) return;

    const characterHealth = document.getElementById('character-health');
    const characterAttack = document.getElementById('character-attack');
    const characterDefense = document.getElementById('character-defense');
    const equippedWeapon = document.getElementById('equipped-weapon');
    const equippedArmor = document.getElementById('equipped-armor');
    const inventory = document.getElementById('inventory');

    // Update character stats
    characterHealth.textContent = player.character.stats.health;
    characterAttack.textContent = player.character.stats.attack;
    characterDefense.textContent = player.character.stats.defense;

    // Update equipment
    if (player.character.equipment.weapon) {
        equippedWeapon.textContent = player.character.equipment.weapon.name;
    } else {
        equippedWeapon.textContent = 'None';
    }

    if (player.character.equipment.armor) {
        equippedArmor.textContent = player.character.equipment.armor.name;
    } else {
        equippedArmor.textContent = 'None';
    }

    // Update inventory
    inventory.innerHTML = '';
    if (player.character.inventory && Object.keys(player.character.inventory).length > 0) {
        Object.values(player.character.inventory).forEach(item => {
            const itemElement = document.createElement('div');
            itemElement.className = 'inventory-item';

            // Check if item is equipped
            const isWeaponEquipped = player.character.equipment.weapon && player.character.equipment.weapon.id === item.id;
            const isArmorEquipped = player.character.equipment.armor && player.character.equipment.armor.id === item.id;

            if (isWeaponEquipped || isArmorEquipped) {
                itemElement.classList.add('equipped');
            }

            itemElement.textContent = `${item.name} (${item.type})`;
            itemElement.addEventListener('click', () => {
                equipItem(game.id, state.playerId, item.id);
            });

            inventory.appendChild(itemElement);
        });
    } else {
        inventory.innerHTML = '<p>No items in inventory</p>';
    }
}

// Update boss info
function updateBossInfo() {
    const game = state.game;
    if (!game || !game.boss) return;

    const bossName = document.getElementById('boss-name');
    const bossHealth = document.getElementById('boss-health');
    const bossMaxHealth = document.getElementById('boss-max-health');
    const bossHealthFill = document.getElementById('boss-health-fill');

    bossName.textContent = game.boss.name;
    bossHealth.textContent = game.boss.health;
    bossMaxHealth.textContent = game.boss.maxHealth;

    // Update boss health bar
    const healthPercentage = (game.boss.health / game.boss.maxHealth) * 100;
    bossHealthFill.style.width = `${healthPercentage}%`;
}

function addEventToLog(eventType, description) {
    const eventLog = document.getElementById('event-log');
    const eventElement = document.createElement('p');
    eventElement.className = `event-${eventType}`;
    eventElement.textContent = description;

    // 이벤트 타입에 따른 스타일 적용
    switch (eventType) {
        case 'player_join':
        case 'player_ready':
            eventElement.style.color = '#4a69bd';
            break;
        case 'player_attack':
            eventElement.style.color = '#e55039';
            break;
        case 'boss_attack':
            eventElement.style.color = '#b71540';
            break;
        case 'player_defeated':
            eventElement.style.color = '#b71540';
            break;
        case 'game_start':
        case 'game_end':
            eventElement.style.fontWeight = 'bold';
            break;
        case 'crafting':
            eventElement.style.color = '#009432';
            break;
        case 'system':
            eventElement.style.color = '#7f8fa6';
            eventElement.style.fontStyle = 'italic';
            break;
    }

    eventLog.appendChild(eventElement);
    eventLog.scrollTop = eventLog.scrollHeight;
}

function showGameResult(game) {
    const resultTitle = document.getElementById('result-title');
    const resultMessage = document.getElementById('result-message');
    const rewardsPanel = document.getElementById('rewards-panel');
    const rewardsList = document.getElementById('rewards-list');

    if (game.result === 'victory') {
        resultTitle.textContent = 'Victory!';
        resultMessage.textContent = `You have defeated the ${game.boss.name}!`;

        // Show rewards
        rewardsPanel.classList.remove('hidden');
        rewardsList.innerHTML = '';

        if (game.rewards && game.rewards[state.playerId]) {
            game.rewards[state.playerId].forEach(reward => {
                const rewardItem = document.createElement('div');
                rewardItem.className = 'reward-item';

                const rewardName = document.createElement('span');
                rewardName.textContent = reward.name;

                const rewardValue = document.createElement('span');
                rewardValue.textContent = reward.value;

                if (reward.type === 'gold') {
                    rewardName.classList.add('reward-gold');
                } else {
                    rewardName.classList.add('reward-item');
                }

                rewardItem.appendChild(rewardName);
                rewardItem.appendChild(rewardValue);
                rewardsList.appendChild(rewardItem);
            });
        } else {
            rewardsList.innerHTML = '<p>No rewards received</p>';
        }
    } else if (game.result === 'defeat') {
        resultTitle.textContent = 'Defeat!';
        resultMessage.textContent = `Your party has been wiped out by the ${game.boss.name}.`;
        rewardsPanel.classList.add('hidden');
    } else {
        resultTitle.textContent = 'Game Over';
        resultMessage.textContent = 'The game has ended.';
        rewardsPanel.classList.add('hidden');
    }

    // Show the results panel
    document.getElementById('results-panel').classList.remove('hidden');
}

// Initialize rooms list
async function initRoomsList() {
    const roomsList = document.getElementById('rooms-list');
    roomsList.innerHTML = '<p>Loading rooms...</p>';

    const rooms = await getRooms();
    console.log('Rooms received:', rooms); // Debug log

    if (!rooms || rooms.length === 0) {
        roomsList.innerHTML = '<p>No rooms available. Create a new room!</p>';
        return;
    }

    roomsList.innerHTML = '';

    rooms.forEach(room => {
        console.log('Processing room:', room); // Debug log

        const roomItem = document.createElement('div');
        roomItem.className = 'room-item';
        roomItem.dataset.roomId = room.id;
        roomItem.dataset.roomName = room.name;

        const roomName = document.createElement('span');
        roomName.className = 'room-name';
        roomName.textContent = room.name;

        const roomStatus = document.createElement('span');
        roomStatus.className = 'room-status';

        // Safely access player count
        let playerCount = room.playerCount || 0;
        roomStatus.textContent = `${playerCount}/3 players`;

        roomItem.appendChild(roomName);
        roomItem.appendChild(roomStatus);

        roomsList.appendChild(roomItem);
    });
}

// Auto-refresh rooms list periodically
let roomsRefreshInterval = null;

function startRoomsRefresh() {
    // Clear any existing interval
    if (roomsRefreshInterval) {
        clearInterval(roomsRefreshInterval);
    }

    // Refresh immediately
    initRoomsList();

    // Set up periodic refresh every 3 seconds
    roomsRefreshInterval = setInterval(() => {
        // Always refresh the rooms list
        initRoomsList();
    }, 3000);
}

function stopRoomsRefresh() {
    if (roomsRefreshInterval) {
        clearInterval(roomsRefreshInterval);
        roomsRefreshInterval = null;
    }
}

// Event listeners
document.addEventListener('DOMContentLoaded', () => {
    // Initialize client with random user ID
    const userInfo = gameClient.initialize();
    console.log('Client initialized with:', userInfo);

    // Try to load from localStorage if not already loaded
    if (!state.playerId && localStorage.getItem('playerId')) {
        state.playerId = localStorage.getItem('playerId');
        state.playerName = localStorage.getItem('playerName') || 'Guest';
        state.gameId = localStorage.getItem('gameId');
        console.log('Loaded user data from localStorage:', {
            playerId: state.playerId,
            playerName: state.playerName,
            gameId: state.gameId
        });
    }

    // Initialize the game status
    updateGameStatus('disconnected', 'Welcome to Boss Raid!');

    // Start auto-refreshing the rooms list
    startRoomsRefresh();

    // Create room button
    document.getElementById('create-room-submit').addEventListener('click', async () => {
        const roomName = document.getElementById('room-name').value.trim();
        const playerName = document.getElementById('player-name').value.trim();

        if (!roomName) {
            alert('Please enter a room name');
            return;
        }

        if (!playerName) {
            alert('Please enter your name');
            return;
        }

        state.playerName = playerName;

        const room = await createRoom(roomName);

        if (room) {
            state.selectedRoom = room;

            // Join the room
            const result = await joinRoom(state.selectedRoom.id, playerName);

            if (!result) {
                return;
            }

            state.gameId = result.game.id;
            state.playerId = result.playerId;
            state.game = result.game;
            state.selectedRoom = result.room;

            // Connect to event source
            connectToEventSource(state.gameId, state.playerId);
            updateGameStatus('waiting', 'Joined room: ' + state.selectedRoom.name);

            // Stop auto-refreshing rooms list when in a game
            stopRoomsRefresh();
        }
    });

    // Refresh rooms button
    document.getElementById('refresh-rooms').addEventListener('click', () => {
        initRoomsList();
    });

    // Ready button
    document.getElementById('ready-btn').addEventListener('click', async () => {
        const result = await readyGame(state.gameId, state.playerId);

        if (result) {
            document.getElementById('ready-btn').disabled = true;
            addEventToLog('system', 'You are ready!');
        }
    });

    // Attack button
    document.getElementById('attack-btn').addEventListener('click', async () => {
        // Manual attack still works, but resets the auto-attack timer
        state.lastAttackTime = Date.now();
        await attackBoss(state.gameId, state.playerId);
    });

    // Add auto-attack indicator to the button
    const attackBtn = document.getElementById('attack-btn');
    attackBtn.innerHTML = 'Attack <span class="auto-attack-indicator">(Auto)</span>';

    // Leave button
    document.getElementById('leave-btn').addEventListener('click', () => {
        disconnectFromEventSource();
        updateGameStatus('disconnected', 'Disconnected from game');
        document.getElementById('current-room-name').textContent = 'None';
        document.getElementById('players-list').innerHTML = '<p>No players connected</p>';
        document.getElementById('inventory').innerHTML = '<p>No items in inventory</p>';
        document.getElementById('results-panel').classList.add('hidden');

        // Restart room refresh when leaving a game
        startRoomsRefresh();
    });

    // Play again button
    document.getElementById('play-again-btn').addEventListener('click', () => {
        disconnectFromEventSource();
        updateGameStatus('disconnected', 'Ready to play again');
        document.getElementById('results-panel').classList.add('hidden');
        document.getElementById('current-room-name').textContent = 'None';
        document.getElementById('players-list').innerHTML = '<p>No players connected</p>';
        document.getElementById('inventory').innerHTML = '<p>No items in inventory</p>';

        // Restart room refresh when playing again
        startRoomsRefresh();
    });

    // Room list click handler
    document.getElementById('rooms-list').addEventListener('click', async (e) => {
        // Check if a room item was clicked
        if (e.target.classList.contains('room-item') || e.target.parentElement.classList.contains('room-item')) {
            const roomItem = e.target.classList.contains('room-item') ? e.target : e.target.parentElement;
            const roomId = roomItem.dataset.roomId;
            const playerName = document.getElementById('player-name').value.trim();

            if (!playerName) {
                alert('Please enter your name');
                return;
            }

            state.playerName = playerName;

            // Join the selected room
            const result = await joinRoom(roomId, playerName);

            if (!result) {
                return;
            }

            state.gameId = result.game.id;
            state.playerId = result.playerId;
            state.game = result.game;
            state.selectedRoom = result.room;

            // Connect to event source
            connectToEventSource(state.gameId, state.playerId);
            updateGameStatus('waiting', 'Joined room: ' + state.selectedRoom.name);

            // Stop auto-refreshing rooms list when in a game
            stopRoomsRefresh();
        }
    });
});
