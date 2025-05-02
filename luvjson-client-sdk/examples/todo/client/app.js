// Todo 앱 클라이언트 코드

// 전역 변수
let todoDocument = null;
let currentFilter = 'all';
let socket = null;
let clientId = `client-${Date.now()}`;
let documentVersion = 0;
let isEditing = false;

// DOM 요소
const newTodoInput = document.getElementById('new-todo');
const addButton = document.getElementById('add-button');
const todoList = document.getElementById('todo-list');
const allFilter = document.getElementById('all-filter');
const activeFilter = document.getElementById('active-filter');
const completedFilter = document.getElementById('completed-filter');
const itemsLeft = document.getElementById('items-left');
const clearCompleted = document.getElementById('clear-completed');
const versionElement = document.getElementById('version');
const syncStatus = document.getElementById('sync-status');
const statusIndicator = document.getElementById('status-indicator');
const statusText = document.getElementById('status-text');

// 초기화
document.addEventListener('DOMContentLoaded', () => {
    initApp();
});

// 앱 초기화
async function initApp() {
    // 이벤트 리스너 등록
    addButton.addEventListener('click', addTodo);
    newTodoInput.addEventListener('keypress', (e) => {
        if (e.key === 'Enter') addTodo();
    });
    allFilter.addEventListener('click', () => setFilter('all'));
    activeFilter.addEventListener('click', () => setFilter('active'));
    completedFilter.addEventListener('click', () => setFilter('completed'));
    clearCompleted.addEventListener('click', clearCompletedTodos);

    // WebSocket 연결
    connectWebSocket();
}

// WebSocket 연결
function connectWebSocket() {
    updateConnectionStatus('offline', '연결 중...');

    // WebSocket 프로토콜 결정 (HTTPS면 WSS, HTTP면 WS)
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    const wsUrl = `${protocol}//${window.location.host}/ws`;

    socket = new WebSocket(wsUrl);

    // 연결 이벤트
    socket.addEventListener('open', () => {
        updateConnectionStatus('online', '온라인');

        // 초기 메시지 전송 (문서 ID)
        socket.send(JSON.stringify({
            documentId: 'todos'
        }));
    });

    // 메시지 수신 이벤트
    socket.addEventListener('message', (event) => {
        const message = JSON.parse(event.data);
        handleServerMessage(message);
    });

    // 연결 종료 이벤트
    socket.addEventListener('close', () => {
        updateConnectionStatus('offline', '오프라인');

        // 재연결 시도
        setTimeout(() => {
            connectWebSocket();
        }, 3000);
    });

    // 오류 이벤트
    socket.addEventListener('error', (error) => {
        console.error('WebSocket error:', error);
        updateConnectionStatus('offline', '연결 오류');
    });
}

// 서버 메시지 처리
function handleServerMessage(message) {
    switch (message.type) {
        case 'init':
            // 초기 문서 상태 처리
            console.log(`Received initial document state:`, message.document);
            console.log(`Initial document version: ${message.document.version}`);
            console.log(`Initial document content:`, message.document.content);
            console.log(`Initial document content keys:`, Object.keys(message.document.content));
            console.log(`Initial document content items count:`, Object.keys(message.document.content).length);

            todoDocument = message.document;
            documentVersion = todoDocument.version;

            // 문서 내용 검증
            if (!todoDocument.content) {
                console.error(`Invalid document content: content is ${todoDocument.content}`);
                todoDocument.content = {};
            }

            updateVersionDisplay();
            renderTodos();

            // 초기화 후 문서 상태 로깅
            console.log(`Document initialized. Version: ${documentVersion}, Items count: ${Object.keys(todoDocument.content).length}`);
            break;

        case 'patch':
            // 패치 적용 (서버에서 받은 버전 정보 포함)
            applyPatch(message.patch, message.version);
            break;

        case 'ack':
            // 확인 메시지 처리
            if (message.success) {
                syncStatus.textContent = `마지막 동기화: ${new Date().toLocaleTimeString()}`;
                // 버전 업데이트 (이미 로컬에서 업데이트했으므로 UI만 업데이트)
                updateVersionDisplay();
            } else {
                console.error('Server ack error:', message.error);
                syncStatus.textContent = `동기화 오류: ${message.error || '알 수 없는 오류'}`;
            }
            break;

        case 'error':
            // 오류 메시지 처리
            console.error('Server error:', message.error);
            syncStatus.textContent = `오류: ${message.error}`;
            break;
    }
}

// 패치 적용 (서버에서 받은 패치)
function applyPatch(patch, serverVersion) {
    console.log(`Applying patch from server. Current version: ${documentVersion}, Server version: ${serverVersion}`);
    console.log(`Patch details:`, patch);

    // 현재 문서 상태 로깅
    console.log(`Current document content:`, todoDocument.content);
    console.log(`Current document content keys:`, Object.keys(todoDocument.content));
    console.log(`Current document content items count:`, Object.keys(todoDocument.content).length);

    // 작업 적용
    for (const op of patch.operations) {
        console.log(`Applying operation:`, op);

        try {
            switch (op.type) {
                case 'add':
                    // Todo 항목 추가
                    try {
                        const newTodo = JSON.parse(op.value);
                        console.log(`Parsed new todo:`, newTodo);
                        todoDocument.content[op.path] = newTodo;
                        console.log(`Added todo item: ${op.path}, title: ${newTodo.title}`);
                    } catch (error) {
                        console.error(`Failed to parse todo item: ${op.value}`, error);
                    }
                    break;

                case 'update':
                    // Todo 항목 업데이트
                    if (!todoDocument.content[op.path]) {
                        console.error(`Todo item not found for update: ${op.path}`);
                        continue;
                    }

                    try {
                        const updates = JSON.parse(op.value);
                        console.log(`Parsed updates:`, updates);
                        Object.assign(todoDocument.content[op.path], updates);
                        console.log(`Updated todo item: ${op.path}`);
                    } catch (error) {
                        console.error(`Failed to parse updates: ${op.value}`, error);
                    }
                    break;

                case 'remove':
                    // Todo 항목 삭제
                    if (!todoDocument.content[op.path]) {
                        console.error(`Todo item not found for removal: ${op.path}`);
                        continue;
                    }

                    delete todoDocument.content[op.path];
                    console.log(`Removed todo item: ${op.path}`);
                    break;

                default:
                    console.error(`Unknown operation type: ${op.type}`);
            }
        } catch (error) {
            console.error(`Error applying operation:`, op, error);
        }
    }

    // 서버 버전으로 문서 버전 설정 (서버 버전이 제공된 경우)
    if (serverVersion !== undefined) {
        documentVersion = serverVersion;
        console.log(`Updated document version to server version: ${serverVersion}`);
    } else {
        // 서버 버전이 제공되지 않은 경우 버전 증가
        documentVersion++;
        console.log(`Incremented document version to: ${documentVersion}`);
    }

    updateVersionDisplay();

    // 업데이트 후 문서 상태 로깅
    console.log(`Updated document content:`, todoDocument.content);
    console.log(`Updated document content keys:`, Object.keys(todoDocument.content));
    console.log(`Updated document content items count:`, Object.keys(todoDocument.content).length);

    // UI 업데이트
    renderTodos();
}

// 로컬 패치 적용 (클라이언트에서 생성한 패치)
function applyLocalPatch(operations) {
    console.log(`Applying local patch. Current version: ${documentVersion}`);
    console.log(`Operations:`, operations);

    // 현재 문서 상태 로깅
    console.log(`Current document content:`, todoDocument.content);
    console.log(`Current document content keys:`, Object.keys(todoDocument.content));
    console.log(`Current document content items count:`, Object.keys(todoDocument.content).length);

    // 작업 적용
    for (const op of operations) {
        console.log(`Applying local operation:`, op);

        try {
            switch (op.type) {
                case 'add':
                    // Todo 항목 추가
                    try {
                        const todoStr = op.value;
                        const newTodo = JSON.parse(todoStr);
                        console.log(`Parsed new todo:`, newTodo);
                        todoDocument.content[op.path] = newTodo;
                        console.log(`Added todo item locally: ${op.path}, title: ${newTodo.title}`);
                    } catch (error) {
                        console.error(`Failed to parse todo item locally: ${op.value}`, error);
                    }
                    break;

                case 'update':
                    // Todo 항목 업데이트
                    if (!todoDocument.content[op.path]) {
                        console.error(`Todo item not found for local update: ${op.path}`);
                        continue;
                    }

                    try {
                        const updatesStr = op.value;
                        const updates = JSON.parse(updatesStr);
                        console.log(`Parsed updates locally:`, updates);
                        Object.assign(todoDocument.content[op.path], updates);
                        console.log(`Updated todo item locally: ${op.path}`);
                    } catch (error) {
                        console.error(`Failed to parse updates locally: ${op.value}`, error);
                    }
                    break;

                case 'remove':
                    // Todo 항목 삭제
                    if (!todoDocument.content[op.path]) {
                        console.error(`Todo item not found for local removal: ${op.path}`);
                        continue;
                    }

                    delete todoDocument.content[op.path];
                    console.log(`Removed todo item locally: ${op.path}`);
                    break;

                default:
                    console.error(`Unknown operation type locally: ${op.type}`);
            }
        } catch (error) {
            console.error(`Error applying local operation:`, op, error);
        }
    }

    // 문서 버전 증가
    documentVersion++;
    console.log(`Incremented document version locally to: ${documentVersion}`);
    updateVersionDisplay();

    // 업데이트 후 문서 상태 로깅
    console.log(`Updated document content locally:`, todoDocument.content);
    console.log(`Updated document content keys locally:`, Object.keys(todoDocument.content));
    console.log(`Updated document content items count locally:`, Object.keys(todoDocument.content).length);

    // UI 업데이트
    renderTodos();
}

// 패치 전송
function sendPatch(operations) {
    if (!socket || socket.readyState !== WebSocket.OPEN) {
        console.error('WebSocket is not connected');
        return;
    }

    const patch = {
        documentId: todoDocument.id,
        baseVersion: documentVersion,
        operations: operations,
        clientId: clientId
    };

    // 패치를 서버로 전송
    socket.send(JSON.stringify(patch));

    // 로컬에도 패치 적용
    applyLocalPatch(operations);
}

// Todo 추가
function addTodo() {
    const title = newTodoInput.value.trim();
    if (!title) return;

    // 새 Todo 항목 생성
    const id = `todo-${Date.now()}`;
    const todo = {
        id: id,
        title: title,
        completed: false,
        createdAt: new Date().toISOString(),
        updatedAt: new Date().toISOString()
    };

    // 작업 생성
    const operation = {
        type: 'add',
        path: id,
        value: JSON.stringify(todo),
        timestamp: Date.now(),
        clientId: clientId
    };

    // 패치 전송
    sendPatch([operation]);

    // 입력 필드 초기화
    newTodoInput.value = '';
}

// Todo 완료 상태 토글
function toggleTodo(id) {
    const todo = todoDocument.content[id];
    if (!todo) return;

    const completed = !todo.completed;

    // 작업 생성
    const operation = {
        type: 'update',
        path: id,
        value: JSON.stringify({ completed: completed }),
        timestamp: Date.now(),
        clientId: clientId
    };

    // 패치 전송
    sendPatch([operation]);
}

// Todo 삭제
function deleteTodo(id) {
    // 작업 생성
    const operation = {
        type: 'remove',
        path: id,
        value: null,
        timestamp: Date.now(),
        clientId: clientId
    };

    // 패치 전송
    sendPatch([operation]);
}

// Todo 편집
function editTodo(id, newTitle) {
    const todo = todoDocument.content[id];
    if (!todo) return;

    // 작업 생성
    const operation = {
        type: 'update',
        path: id,
        value: JSON.stringify({ title: newTitle }),
        timestamp: Date.now(),
        clientId: clientId
    };

    // 패치 전송
    sendPatch([operation]);
}

// 완료된 Todo 모두 삭제
function clearCompletedTodos() {
    const operations = [];

    // 완료된 항목 찾기
    for (const id in todoDocument.content) {
        if (todoDocument.content[id].completed) {
            operations.push({
                type: 'remove',
                path: id,
                value: null,
                timestamp: Date.now(),
                clientId: clientId
            });
        }
    }

    if (operations.length > 0) {
        // 패치 전송
        sendPatch(operations);
    }
}

// 필터 설정
function setFilter(filter) {
    currentFilter = filter;

    // 필터 버튼 활성화 상태 업데이트
    allFilter.classList.toggle('active', filter === 'all');
    activeFilter.classList.toggle('active', filter === 'active');
    completedFilter.classList.toggle('active', filter === 'completed');

    // Todo 목록 다시 렌더링
    renderTodos();
}

// Todo 목록 렌더링
function renderTodos() {
    // 목록 초기화
    todoList.innerHTML = '';

    // 필터링된 Todo 항목 가져오기
    const todos = Object.values(todoDocument.content);
    const filteredTodos = todos.filter(todo => {
        if (currentFilter === 'active') return !todo.completed;
        if (currentFilter === 'completed') return todo.completed;
        return true; // 'all' 필터
    });

    // 남은 항목 수 업데이트
    const activeCount = todos.filter(todo => !todo.completed).length;
    itemsLeft.textContent = `${activeCount} 항목 남음`;

    // Todo 항목 렌더링
    filteredTodos.forEach(todo => {
        const li = document.createElement('li');
        li.className = `todo-item ${todo.completed ? 'completed' : ''}`;
        li.dataset.id = todo.id;

        // 체크박스
        const checkbox = document.createElement('input');
        checkbox.type = 'checkbox';
        checkbox.className = 'todo-checkbox';
        checkbox.checked = todo.completed;
        checkbox.addEventListener('change', () => toggleTodo(todo.id));

        // 텍스트
        const span = document.createElement('span');
        span.className = 'todo-text';
        span.textContent = todo.title;
        span.addEventListener('dblclick', () => {
            if (!isEditing) {
                isEditing = true;
                const input = document.createElement('input');
                input.className = 'edit-input';
                input.value = todo.title;
                li.replaceChild(input, span);
                input.focus();

                // 포커스 잃을 때 편집 완료
                input.addEventListener('blur', () => {
                    const newTitle = input.value.trim();
                    if (newTitle && newTitle !== todo.title) {
                        editTodo(todo.id, newTitle);
                    }
                    li.replaceChild(span, input);
                    isEditing = false;
                });

                // Enter 키 누를 때 편집 완료
                input.addEventListener('keypress', (e) => {
                    if (e.key === 'Enter') {
                        input.blur();
                    }
                });
            }
        });

        // 삭제 버튼
        const deleteBtn = document.createElement('button');
        deleteBtn.className = 'todo-delete';
        deleteBtn.innerHTML = '&times;';
        deleteBtn.addEventListener('click', () => deleteTodo(todo.id));

        // 요소 추가
        li.appendChild(checkbox);
        li.appendChild(span);
        li.appendChild(deleteBtn);
        todoList.appendChild(li);
    });
}

// 버전 표시 업데이트
function updateVersionDisplay() {
    versionElement.textContent = `버전: ${documentVersion}`;
}

// 연결 상태 업데이트
function updateConnectionStatus(status, text) {
    statusIndicator.className = status;
    statusText.textContent = text;
}
