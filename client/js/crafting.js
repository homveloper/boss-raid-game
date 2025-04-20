/**
 * 협동 아이템 제작 시스템 클라이언트 코드
 */

// 제작 시스템 클래스
class CraftingSystem {
    constructor(gameClient) {
        this.gameClient = gameClient;
        this.craftableItems = {};
        this.craftingItems = {};
        this.lastUpdated = new Date();
        this.updateInterval = null;
    }

    // 초기화
    initialize() {
        // 게임 클라이언트가 초기화되었는지 확인
        if (!this.gameClient.playerId) {
            console.error('Game client not initialized. Initializing now...');
            this.gameClient.initialize();
        }

        console.log('Crafting system initializing with:', {
            gameId: this.gameClient.gameId,
            playerId: this.gameClient.playerId
        });

        // 제작 가능한 아이템 목록 로드
        this.loadCraftableItems();

        // 제작 중인 아이템 목록 로드
        this.loadCraftingItems();

        // 주기적으로 제작 상태 업데이트 (10초마다)
        // SSE를 통해 실시간 업데이트가 되지만, 완료 상태 갱신을 위해 주기적 업데이트도 유지
        this.updateInterval = setInterval(() => {
            this.loadCraftingItems();
        }, 10000);

        // 이벤트 리스너 등록
        this.registerEventListeners();
    }

    // 이벤트 리스너 등록
    registerEventListeners() {
        // 게임 이벤트 리스너
        document.addEventListener('game_event', (e) => {
            const event = e.detail;

            // 제작 관련 이벤트 처리
            if (event.type === 'crafting_started') {
                this.handleCraftingStarted(event);
            } else if (event.type === 'crafting_helped') {
                this.handleCraftingHelped(event);
            } else if (event.type === 'crafting_completed') {
                this.handleCraftingCompleted(event);
            }
        });

        // SSE를 통한 실시간 제작 상태 업데이트 이벤트 리스너
        document.addEventListener('crafting_update', (e) => {
            console.log('Received crafting_update event:', e.detail);
            this.handleCraftingUpdate(e.detail);
        });
    }

    // 제작 시작 이벤트 처리
    handleCraftingStarted(event) {
        console.log('Crafting started:', event);
        // 제작 중인 아이템 목록 새로고침
        this.loadCraftingItems();
    }

    // 제작 도움 이벤트 처리
    handleCraftingHelped(event) {
        console.log('Crafting helped:', event);
        // 제작 중인 아이템 목록 새로고침
        this.loadCraftingItems();
    }

    // 제작 완료 이벤트 처리
    handleCraftingCompleted(event) {
        console.log('Crafting completed:', event);
        // 제작 중인 아이템 목록 새로고침
        this.loadCraftingItems();

        // 알림 표시
        this.gameClient.showNotification(`${event.payload.data.crafterName}의 아이템 제작이 완료되었습니다!`);
    }

    // SSE를 통한 실시간 제작 상태 업데이트 처리
    handleCraftingUpdate(data) {
        console.log('Handling crafting update:', data);

        // 업데이트 타입에 따른 처리
        if (data.type === 'start') {
            // 제작 시작 이벤트
            const craftingItem = {
                id: data.craftingId,
                itemId: data.itemId,
                crafterID: data.playerId,
                crafterName: data.playerName,
                startTime: new Date(data.startTime),
                originalTimeMinutes: data.originalTime,
                currentTimeMinutes: data.currentTime,
                helpers: {},
                status: 'in_progress'
            };

            // 제작 중인 아이템 목록에 추가
            this.craftingItems[craftingItem.id] = craftingItem;

            // UI 업데이트
            this.updateCraftingItemsUI();

            // 내가 시작한 제작이 아니면 알림 표시
            if (data.playerId !== this.gameClient.playerId) {
                this.gameClient.showNotification(`${data.playerName}님이 ${data.itemName} 제작을 시작했습니다.`);
            }
        } else if (data.type === 'help') {
            // 제작 도움 이벤트
            const craftingItem = this.craftingItems[data.craftingId];
            if (craftingItem) {
                // 도움 정보 추가
                if (!craftingItem.helpers[data.helperId]) {
                    craftingItem.helpers[data.helperId] = 0;
                }
                craftingItem.helpers[data.helperId]++;

                // 제작 시간 업데이트
                craftingItem.currentTimeMinutes = data.currentTime;

                // UI 업데이트
                this.updateCraftingItemsUI();

                // 내가 도움을 준 것이 아니면 알림 표시
                if (data.helperId !== this.gameClient.playerId) {
                    this.gameClient.showNotification(`${data.helperName}님이 ${data.crafterName}님의 제작을 도와주었습니다.`);
                }
            }
        }
    }

    // 제작 가능한 아이템 목록 로드
    async loadCraftableItems() {
        try {
            const gameId = this.gameClient.gameId;
            if (!gameId) return;

            const response = await fetch(`/api/crafting/items?gameId=${gameId}`);
            if (!response.ok) {
                throw new Error(`Failed to load craftable items: ${response.statusText}`);
            }

            const data = await response.json();
            this.craftableItems = {};

            // 아이템 목록 저장
            data.craftableItems.forEach(item => {
                this.craftableItems[item.id] = item;
            });

            // UI 업데이트
            this.updateCraftableItemsUI();

            return this.craftableItems;
        } catch (error) {
            console.error('Error loading craftable items:', error);
        }
    }

    // 제작 중인 아이템 목록 로드
    async loadCraftingItems() {
        try {
            const gameId = this.gameClient.gameId;
            if (!gameId) return;

            const response = await fetch(`/api/crafting/in-progress?gameId=${gameId}`);
            if (!response.ok) {
                throw new Error(`Failed to load crafting items: ${response.statusText}`);
            }

            const data = await response.json();
            this.craftingItems = {};

            // 아이템 목록 저장
            data.craftingItems.forEach(item => {
                this.craftingItems[item.id] = item;
            });

            // UI 업데이트
            this.updateCraftingItemsUI();

            return this.craftingItems;
        } catch (error) {
            console.error('Error loading crafting items:', error);
        }
    }

    // 아이템 제작 시작
    async startCrafting(itemId) {
        try {
            const gameId = this.gameClient.gameId;
            const playerId = this.gameClient.playerId;

            if (!gameId || !playerId) {
                throw new Error('Game ID or Player ID is missing');
            }

            const response = await fetch('/api/crafting/start', {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json'
                },
                body: JSON.stringify({
                    gameId: gameId,
                    playerId: playerId,
                    itemId: itemId
                })
            });

            if (!response.ok) {
                const errorData = await response.json();
                throw new Error(errorData.message || `Failed to start crafting: ${response.statusText}`);
            }

            // 제작 중인 아이템 목록 새로고침
            await this.loadCraftingItems();

            return true;
        } catch (error) {
            console.error('Error starting crafting:', error);
            this.gameClient.showNotification(`제작 시작 실패: ${error.message}`, 'error');
            return false;
        }
    }

    // 제작 도움
    async helpCrafting(craftingId) {
        try {
            const gameId = this.gameClient.gameId;
            const playerId = this.gameClient.playerId;

            if (!gameId || !playerId) {
                throw new Error('Game ID or Player ID is missing');
            }

            const response = await fetch('/api/crafting/help', {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json'
                },
                body: JSON.stringify({
                    gameId: gameId,
                    playerId: playerId,
                    craftingId: craftingId
                })
            });

            if (!response.ok) {
                const errorData = await response.json();
                throw new Error(errorData.message || `Failed to help crafting: ${response.statusText}`);
            }

            // 제작 중인 아이템 목록 새로고침
            await this.loadCraftingItems();

            return true;
        } catch (error) {
            console.error('Error helping crafting:', error);
            this.gameClient.showNotification(`도움 제공 실패: ${error.message}`, 'error');
            return false;
        }
    }

    // 제작 진행 상태 계산
    getCraftingProgress(craftingItem) {
        if (!craftingItem) return { progress: 0, remainingMinutes: 0, completed: false };

        // 이미 완료된 경우
        if (craftingItem.status === "completed") {
            return { progress: 100, remainingMinutes: 0, completed: true };
        }

        // 취소된 경우
        if (craftingItem.status === "cancelled") {
            return { progress: 0, remainingMinutes: 0, cancelled: true };
        }

        // 진행 중인 경우
        const startTime = new Date(craftingItem.startTime);
        const totalMinutes = craftingItem.currentTimeMinutes;
        const elapsedMinutes = (new Date() - startTime) / (1000 * 60);
        const remainingMinutes = Math.max(0, totalMinutes - elapsedMinutes);

        // 진행률 계산 (0-100%)
        const progress = Math.min(100, (elapsedMinutes / totalMinutes) * 100);

        // 클라이언트 측에서 완료 여부 확인
        const completed = remainingMinutes <= 0;

        return {
            progress: progress,
            remainingMinutes: remainingMinutes,
            completed: completed,
            helpers: craftingItem.helpers
        };
    }

    // 제작 가능한 아이템 목록 UI 업데이트
    updateCraftableItemsUI() {
        const container = document.getElementById('craftable-items-container');
        if (!container) return;

        // 컨테이너 초기화
        container.innerHTML = '';

        // 제목 추가
        const title = document.createElement('h3');
        title.textContent = '제작 가능한 아이템';
        title.className = 'crafting-title';
        container.appendChild(title);

        // 아이템 목록 컨테이너
        const itemsGrid = document.createElement('div');
        itemsGrid.className = 'craftable-items-grid';
        container.appendChild(itemsGrid);

        // 아이템이 없는 경우
        if (Object.keys(this.craftableItems).length === 0) {
            const emptyMessage = document.createElement('p');
            emptyMessage.textContent = '제작 가능한 아이템이 없습니다.';
            emptyMessage.className = 'empty-message';
            itemsGrid.appendChild(emptyMessage);
            return;
        }

        // 각 아이템에 대한 UI 요소 생성
        Object.values(this.craftableItems).forEach(item => {
            const itemElement = document.createElement('div');
            itemElement.className = 'craftable-item';
            itemElement.dataset.itemId = item.id;

            // 아이템 내용
            itemElement.innerHTML = `
                <div class="item-header">
                    <h4>${item.name}</h4>
                </div>
                <div class="item-body">
                    <p class="item-description">${item.description}</p>
                    <p class="item-time">제작 시간: ${item.baseTimeMinutes}분</p>
                    <p class="item-materials">재료: ${item.materials.join(', ')}</p>
                </div>
                <div class="item-footer">
                    <button class="start-craft-btn" data-item-id="${item.id}">제작 시작</button>
                </div>
            `;

            // 아이템 요소 추가
            itemsGrid.appendChild(itemElement);

            // 제작 시작 버튼 이벤트 리스너
            const startButton = itemElement.querySelector('.start-craft-btn');
            startButton.addEventListener('click', () => {
                this.startCrafting(item.id);
            });
        });
    }

    // 제작 중인 아이템 목록 UI 업데이트
    updateCraftingItemsUI() {
        const container = document.getElementById('crafting-items-container');
        if (!container) return;

        // 컨테이너 초기화
        container.innerHTML = '';

        // 제목 추가
        const title = document.createElement('h3');
        title.textContent = '제작 중인 아이템';
        title.className = 'crafting-title';
        container.appendChild(title);

        // 아이템 목록 컨테이너
        const itemsList = document.createElement('div');
        itemsList.className = 'crafting-items-list';
        container.appendChild(itemsList);

        // 아이템이 없는 경우
        if (Object.keys(this.craftingItems).length === 0) {
            const emptyMessage = document.createElement('p');
            emptyMessage.textContent = '제작 중인 아이템이 없습니다.';
            emptyMessage.className = 'empty-message';
            itemsList.appendChild(emptyMessage);
            return;
        }

        // 각 아이템에 대한 UI 요소 생성
        Object.values(this.craftingItems).forEach(item => {
            // 해당 아이템 정보 가져오기
            const craftableItem = this.craftableItems[item.itemId];
            if (!craftableItem) return;

            // 진행 상태 계산
            const progress = this.getCraftingProgress(item);

            // 아이템 요소 생성
            const itemElement = document.createElement('div');
            itemElement.className = `crafting-item ${progress.completed ? 'completed' : ''} ${item.crafterID === this.gameClient.playerId ? 'my-item' : ''}`;
            itemElement.dataset.craftingId = item.id;

            // 도움 준 플레이어 목록 생성
            const helpersHtml = Object.entries(item.helpers)
                .map(([id, count]) => {
                    const playerName = this.gameClient.getPlayerName(id) || id;
                    return `<span class="helper">${playerName} (${count}회)</span>`;
                })
                .join('') || '<span class="no-helpers">아직 없음</span>';

            // 남은 시간 포맷팅
            const remainingTime = progress.completed ?
                '완료!' :
                `${Math.ceil(progress.remainingMinutes)}분 남음`;

            // 아이템 내용
            itemElement.innerHTML = `
                <div class="crafting-item-header">
                    <h4>${craftableItem.name}</h4>
                    <span class="crafter">제작자: ${item.crafterName}</span>
                </div>
                <div class="crafting-item-body">
                    <div class="progress-container">
                        <div class="progress-bar">
                            <div class="progress" style="width: ${progress.progress}%"></div>
                        </div>
                        <span class="remaining-time">${remainingTime}</span>
                    </div>
                    <div class="time-info">
                        <span>원래 시간: ${item.originalTimeMinutes}분</span>
                        <span>현재 시간: ${item.currentTimeMinutes}분</span>
                    </div>
                    <div class="helpers-container">
                        <h5>도움 준 플레이어:</h5>
                        <div class="helpers-list">${helpersHtml}</div>
                    </div>
                </div>
                <div class="crafting-item-footer">
                    ${item.status !== 'completed' && item.crafterID !== this.gameClient.playerId ?
                        `<button class="help-craft-btn" data-crafting-id="${item.id}">도움 주기</button>` : ''}
                </div>
            `;

            // 아이템 요소 추가
            itemsList.appendChild(itemElement);

            // 도움 주기 버튼 이벤트 리스너
            const helpButton = itemElement.querySelector('.help-craft-btn');
            if (helpButton) {
                helpButton.addEventListener('click', () => {
                    this.helpCrafting(item.id);
                });
            }
        });
    }

    // 정리
    cleanup() {
        if (this.updateInterval) {
            clearInterval(this.updateInterval);
            this.updateInterval = null;
        }
    }
}

// 전역 객체로 내보내기
window.CraftingSystem = CraftingSystem;
