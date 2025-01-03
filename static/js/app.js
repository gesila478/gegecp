// Vue 应用实例
new Vue({
    el: '#app',
    delimiters: ['[[', ']]'],
    data: {
        isLoggedIn: false,
        loginForm: {
            username: '',
            password: ''
        },
        currentView: 'system',
        isMaximized: false,
        systemInfo: {
            cpu: {
                percent: 0,
                model: '',
                trend: 0
            },
            memory: {
                total: 0,
                used: 0,
                free: 0,
                trend: 0
            },
            disk: {
                total: 0,
                used: 0,
                free: 0,
                trend: 0
            }
        },
        processes: [],
        token: localStorage.getItem('token') || '',
        files: [],
        currentPath: '/',
        isEditing: false,
        currentEditingFile: null,
        editor: null,
        isEditorMaximized: false,
        // SSH终端相关
        sshConfig: {
            host: '',
            user: '',
            password: ''
        },
        sshConnected: false,
        terminal: null,
        ws: null,
        // 用户菜单相关
        showUserMenu: false,
        showChangePassword: false,
        passwordForm: {
            oldPassword: '',
            newPassword: '',
            confirmPassword: ''
        },
        // 收藏相关
        favorites: [],
        // 权限设置相关
        showPermissions: false,
        currentFile: null,
        permissions: {
            owner: {
                read: false,
                write: false,
                execute: false
            },
            group: {
                read: false,
                write: false,
                execute: false
            },
            others: {
                read: false,
                write: false,
                execute: false
            },
            recursive: false
        },
    },
    computed: {
        pathParts() {
            const parts = this.currentPath.split('/').filter(Boolean);
            const result = [{ name: 'Root', path: '/' }];
            let currentPath = '';
            
            for (const part of parts) {
                currentPath += '/' + part;
                result.push({
                    name: part,
                    path: currentPath
                });
            }
            
            return result;
        },
        permissionString() {
            const getPermStr = (perm) => {
                return (perm.read ? 'r' : '-') + 
                       (perm.write ? 'w' : '-') + 
                       (perm.execute ? 'x' : '-');
            };
            return getPermStr(this.permissions.owner) + 
                   getPermStr(this.permissions.group) + 
                   getPermStr(this.permissions.others);
        }
    },
    methods: {
        // 通用请求方法
        async request(url, options = {}) {
            const headers = {
                'Authorization': this.token ? `Bearer ${this.token}` : '',
                ...options.headers
            };
            return axios({
                url: '/api' + url,
                ...options,
                headers
            }).then(response => {
                if (options.responseType === 'blob') {
                    return response.data;
                }
                return response.data;
            }).catch(error => {
                console.error('请求错误:', error);
                if (error.response?.status === 401) {
                    this.logout();
                }
                throw error;
            });
        },

        // 登录相关
        async login() {
            try {
                // 对密码进行 MD5 哈希
                const md5Password = CryptoJS.MD5(this.loginForm.password).toString();
                console.log('登录请求详情：', {
                    url: '/api/login',
                    username: this.loginForm.username,
                    passwordHash: md5Password
                });
                
                const response = await axios.post('/api/login', {
                    username: this.loginForm.username,
                    password: md5Password
                });
                
                console.log('登录响应详情：', response);
                
                const data = response.data;
                if (data.token) {
                    localStorage.setItem('token', data.token);
                    this.token = data.token;
                    this.isLoggedIn = true;
                    await this.getSystemInfo();
                } else {
                    throw new Error(data.error || '登录失败');
                }
            } catch (error) {
                console.error('登录错误详情:', {
                    status: error.response?.status,
                    statusText: error.response?.statusText,
                    data: error.response?.data,
                    headers: error.response?.headers,
                    config: error.config
                });
                alert('登录失败：' + (error.response?.data?.error || '用户名或密码错误'));
            }
        },

        logout() {
            localStorage.removeItem('token');
            this.isLoggedIn = false;
            this.token = '';
            this.currentView = 'system';
            this.systemInfo = {
                cpu: { percent: 0, model: '', trend: 0 },
                memory: { total: 0, used: 0, free: 0, trend: 0 },
                disk: { total: 0, used: 0, free: 0, trend: 0 }
            };
            this.processes = [];
            this.files = [];
            this.currentPath = '/';
            this.isEditing = false;
            this.currentEditingFile = null;
            
            if (this.terminal) {
                this.terminal.dispose();
                this.terminal = null;
            }
            if (this.socket) {
                this.socket.close();
                this.socket = null;
            }
            if (this.editor) {
                this.editor.dispose();
                this.editor = null;
            }
        },

        // 系统信息
        async getSystemInfo() {
            try {
                const response = await this.request('/system/info');
                // 计算趋势
                const oldCpuPercent = Number(this.systemInfo.cpu.percent) || 0;
                const oldMemoryPercent = this.systemInfo.memory.used / this.systemInfo.memory.total * 100 || 0;
                const oldDiskPercent = this.systemInfo.disk.used / this.systemInfo.disk.total * 100 || 0;

                this.systemInfo = {
                    cpu: {
                        percent: Number(response.cpu.percent || 0),
                        model: response.cpu.model || '',
                        trend: Number(response.cpu.percent - oldCpuPercent || 0)
                    },
                    memory: {
                        total: Number(response.memory.total || 0),
                        used: Number(response.memory.used || 0),
                        free: Number(response.memory.free || 0),
                        trend: Number((response.memory.used / response.memory.total * 100) - oldMemoryPercent || 0)
                    },
                    disk: {
                        total: Number(response.disk.total || 0),
                        used: Number(response.disk.used || 0),
                        free: Number(response.disk.free || 0),
                        trend: Number((response.disk.used / response.disk.total * 100) - oldDiskPercent || 0)
                    }
                };

                // 更新进程列表
                this.processes = await this.request('/process/list');

                setTimeout(() => this.getSystemInfo(), 5000);
            } catch (error) {
                console.error('获取系统信息失败:', error);
            }
        },

        // 文件管理
        async listFiles() {
            try {
                this.files = await this.request('/files/list', {
                    params: { path: this.currentPath }
                });
            } catch (error) {
                console.error('获取文件列表失败:', error);
            }
        },

        uploadFile() {
            // 触发文件选择框
            this.$refs.fileInput.click();
        },

        handleFileUpload(event) {
            const file = event.target.files[0];
            if (!file) return;

            const formData = new FormData();
            formData.append('file', file);
            formData.append('path', this.currentPath);

            this.request('/files/upload', {
                method: 'POST',
                data: formData,
                headers: {
                    'Content-Type': 'multipart/form-data'
                }
            }).then(() => {
                this.listFiles();
            }).catch(error => {
                console.error('上传文件失败:', error);
                alert('上传文件失败: ' + error.message);
            });
        },

        async deleteFile(file) {
            if (!confirm(`确定要删除 ${file.name} 吗？`)) return;

            try {
                await this.request('/files/delete', {
                    method: 'DELETE',
                    params: { path: this.currentPath + '/' + file.name }
                });
                await this.listFiles();
            } catch (error) {
                console.error('删除文件失败:', error);
            }
        },

        async downloadFile(file) {
            try {
                const blob = await this.request('/files/download', {
                    responseType: 'blob',
                    params: { path: this.currentPath + '/' + file.name }
                });
                const url = window.URL.createObjectURL(blob);
                const a = document.createElement('a');
                a.href = url;
                a.download = file.name;
                document.body.appendChild(a);
                a.click();
                window.URL.revokeObjectURL(url);
                document.body.removeChild(a);
            } catch (error) {
                console.error('下载文件失败:', error);
            }
        },

        // 文件编辑
        async editFile(file) {
            try {
                // 使用文件的绝对路径
                const filePath = file.path || (this.currentPath + (this.currentPath.endsWith('/') ? '' : '/') + file.name);
                const response = await this.request('/files/read', {
                    params: { path: filePath }
                });

                this.currentEditingFile = {
                    name: file.name,
                    path: filePath,
                    content: response
                };

                this.isEditing = true;
                this.$nextTick(() => {
                    if (!this.editor) {
                        this.initEditor();
                    }
                    this.editor.setValue(response);
                    // 自动检测文件类型
                    const model = this.editor.getModel();
                    if (model) {
                        monaco.editor.setModelLanguage(model, this.getFileLanguage(file.name));
                    }
                });
            } catch (error) {
                console.error('读取文件失败:', error);
                alert('读取文件失败: ' + (error.response?.data?.error || error.message));
            }
        },

        async saveFile() {
            if (!this.currentEditingFile || !this.editor) return;

            try {
                const content = this.editor.getValue();
                await this.request('/files/save', {
                    method: 'POST',
                    data: {
                        path: this.currentPath + '/' + this.currentEditingFile.name,
                        content: content
                    }
                });
                this.cancelEdit();
                await this.listFiles();
            } catch (error) {
                console.error('保存文件失败:', error);
                alert('保存失败：' + error.message);
            }
        },

        cancelEdit() {
            if (this.editor) {
                this.editor.dispose();
                this.editor = null;
            }
            this.isEditing = false;
            this.currentEditingFile = null;
        },

        // 文件夹导航
        handleFileClick(file) {
            if (file.isDir) {
                this.navigateToDirectory(file.name);
            } else {
                this.editFile(file);
            }
        },

        navigateToDirectory(dirName) {
            if (dirName === 'Root') {
                this.currentPath = '/';
            } else {
                this.currentPath = this.currentPath === '/' 
                    ? '/' + dirName 
                    : this.currentPath + '/' + dirName;
            }
            this.listFiles();
        },

        navigateTo(path) {
            this.currentPath = path;
            this.listFiles();
        },

        navigateUp() {
            const parts = this.currentPath.split('/').filter(Boolean);
            parts.pop();
            this.currentPath = parts.length === 0 ? '/' : '/' + parts.join('/');
            this.listFiles();
        },

        // 工具方法
        formatBytes(bytes) {
            if (!bytes && bytes !== 0) return '0 B';
            const k = 1024;
            const sizes = ['B', 'KB', 'MB', 'GB', 'TB'];
            const i = Math.floor(Math.log(Math.abs(bytes)) / Math.log(k));
            return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i];
        },

        formatPercent(value) {
            if (value === null || value === undefined || isNaN(value)) {
                return '0%';
            }
            const num = parseFloat(value);
            return num.toFixed(1) + '%';
        },

        formatDate(date) {
            return new Date(date).toLocaleString();
        },

        getFileLanguage(filename) {
            const ext = filename.split('.').pop().toLowerCase();
            const languageMap = {
                'js': 'javascript',
                'py': 'python',
                'html': 'html',
                'css': 'css',
                'json': 'json',
                'md': 'markdown',
                'sh': 'shell',
                'bash': 'shell',
                'txt': 'plaintext',
                'log': 'plaintext',
                'yml': 'yaml',
                'yaml': 'yaml',
                'go': 'go',
                'rs': 'rust',
                'php': 'php',
                'java': 'java',
                'cpp': 'cpp',
                'c': 'c',
                'h': 'c',
                'hpp': 'cpp',
                'sql': 'sql',
                'xml': 'xml'
            };
            return languageMap[ext] || 'plaintext';
        },

        // 进程管理
        async listProcesses() {
            try {
                const data = await this.request('/process/list');
                // 格式化进程数据
                this.processes = data.map(proc => ({
                    name: proc.name || '',
                    pid: proc.pid || 0,
                    cpu: this.formatPercent(proc.cpu_percent || 0),
                    memory: this.formatBytes(proc.memory || 0),
                    status: proc.status?.toLowerCase() || 'unknown'
                }));
            } catch (error) {
                console.error('获取进程列表失败:', error);
                this.processes = [];
            }
        },

        async killProcess(proc) {
            if (!confirm(`确定要终止进程 ${proc.name} (PID: ${proc.pid}) 吗？`)) return;
            
            try {
                await this.request('/process/kill', {
                    method: 'POST',
                    data: { pid: proc.pid }
                });
                await this.listProcesses();
            } catch (error) {
                console.error('终止进程失败:', error);
                alert('终止进程失败: ' + error.message);
            }
        },

        // SSH终端相关方法
        async connectSSH() {
            // 确保之前的连接已经完全清理
            this.disconnectSSH();

            // 验证必要参数
            if (!this.sshConfig.host || this.sshConfig.host.trim() === '') {
                alert('请输入主机地址');
                return;
            }
            if (!this.sshConfig.user || this.sshConfig.user.trim() === '') {
                alert('请输入用户名');
                return;
            }
            if (!this.sshConfig.password || this.sshConfig.password.trim() === '') {
                alert('请输入密码');
                return;
            }

            // 清理参数
            this.sshConfig.host = this.sshConfig.host.trim();
            this.sshConfig.user = this.sshConfig.user.trim();
            this.sshConfig.password = this.sshConfig.password.trim();

            console.log('开始SSH连接:', {
                host: this.sshConfig.host,
                user: this.sshConfig.user,
                hasPassword: !!this.sshConfig.password
            });

            // 初始化xterm.js终端
            this.terminal = new Terminal({
                cursorBlink: true,
                theme: {
                    background: '#1e1e1e',
                    foreground: '#ffffff'
                },
                fontSize: 14,
                fontFamily: 'Menlo, Monaco, Consolas, monospace',
                rows: 30,
                cols: 120
            });

            // 创建并加载FitAddon
            const fitAddon = new window.FitAddon.FitAddon();
            this.terminal.loadAddon(fitAddon);
            this.fitAddon = fitAddon;

            // 打开终端
            const terminalElement = document.getElementById('terminal');
            terminalElement.style.display = 'block';  // 确保终端元素可见
            this.terminal.open(terminalElement);
            this.fitAddon.fit();

            // 监听窗口大小变化
            const resizeHandler = () => {
                if (this.fitAddon) {
                    this.fitAddon.fit();
                }
            };
            window.addEventListener('resize', resizeHandler);

            // 建立WebSocket连接
            const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
            
            // 调试信息
            console.log('SSH配置:', {
                host: this.sshConfig.host,
                user: this.sshConfig.user,
                hasPassword: !!this.sshConfig.password,
                token: this.token
            });

            // 确保所有参数都经过编码
            const params = new URLSearchParams({
                host: this.sshConfig.host,
                username: this.sshConfig.user,
                password: this.sshConfig.password,
                token: this.token
            });

            const wsUrl = `${protocol}//${window.location.host}/api/terminal/ws?${params.toString()}`;
            
            console.log('WebSocket URL (密码已隐藏):', wsUrl.replace(/password=[^&]+/, 'password=***'));

            try {
                this.ws = new WebSocket(wsUrl);

                this.ws.onopen = () => {
                    console.log('WebSocket连接已建立');
                    this.sshConnected = true;
                    this.terminal.clear();
                    // 连接成功后自动聚焦
                    this.terminal.focus();
                };

                this.ws.onmessage = (event) => {
                    // 检查是否是错误消息
                    if (event.data.includes('SSH连接失败') || 
                        event.data.includes('创建SSH会话失败') || 
                        event.data.includes('启动shell失败')) {
                        this.disconnectSSH();
                        this.terminal.write('\x1b[31m' + event.data + '\x1b[0m\r\n'); // 红色显示错误
                        return;
                    }
                    this.terminal.write(event.data);
                };

                this.ws.onclose = (event) => {
                    this.disconnectSSH();
                    const terminalElement = document.getElementById('terminal');
                    if (terminalElement) {
                        if (event.code === 1006) {
                            terminalElement.innerHTML = '<div class="terminal-error">连接异常断开，请检查网络连接或服务器状态。</div>';
                        } else {
                            terminalElement.innerHTML = '<div class="terminal-info">连接已关闭。</div>';
                        }
                    }
                    // 移除resize事件监听
                    window.removeEventListener('resize', resizeHandler);
                };

                this.ws.onerror = (error) => {
                    console.error('WebSocket错误:', error);
                    this.disconnectSSH();
                    const terminalElement = document.getElementById('terminal');
                    if (terminalElement) {
                        terminalElement.innerHTML = '<div class="terminal-error">连接错误：无法连接到终端，请检查服务状态。</div>';
                    }
                };

                // 处理终端输入
                this.terminal.onData(data => {
                    if (this.ws && this.ws.readyState === WebSocket.OPEN) {
                        this.ws.send(data);
                    }
                });

                // 处理终端大小调整
                this.terminal.onResize(size => {
                    if (this.ws && this.ws.readyState === WebSocket.OPEN) {
                        // 发送终端大小调整消息
                        this.ws.send(JSON.stringify({
                            type: 'resize',
                            cols: size.cols,
                            rows: size.rows
                        }));
                    }
                });
            } catch (error) {
                console.error('WebSocket连接错误:', error);
                this.disconnectSSH();
                const terminalElement = document.getElementById('terminal');
                if (terminalElement) {
                    terminalElement.innerHTML = `<div class="terminal-error">连接错误：${error.message}</div>`;
                }
            }
        },

        disconnectSSH() {
            if (this.ws) {
                this.ws.close();
                this.ws = null;
            }
            if (this.terminal) {
                this.terminal.dispose();
                this.terminal = null;
            }
            // 重置所有状态
            this.sshConnected = false;
            this.fitAddon = null;
            // 清空终端元素
            const terminalElement = document.getElementById('terminal');
            if (terminalElement) {
                terminalElement.innerHTML = '';
            }
        },

        // 窗口控制方法
        minimizeWindow() {
            document.documentElement.style.display = 'none';
            setTimeout(() => {
                document.documentElement.style.display = '';
            }, 100);
        },

        toggleMaximize() {
            if (!this.isMaximized) {
                document.documentElement.style.width = '100vw';
                document.documentElement.style.height = '100vh';
                document.body.style.minWidth = 'unset';
                document.body.style.width = '100%';
                document.body.style.height = '100%';
            } else {
                document.documentElement.style.width = '';
                document.documentElement.style.height = '';
                document.body.style.minWidth = '1200px';
                document.body.style.width = '';
                document.body.style.height = '';
            }
            this.isMaximized = !this.isMaximized;
            document.querySelector('.window-btn.maximize').classList.toggle('is-maximized');
        },

        // 编辑器窗口控制
        minimizeEditor() {
            const container = document.querySelector('.modal-container');
            container.style.transform = 'scale(0.1)';
            container.style.opacity = '0';
            setTimeout(() => {
                container.style.transform = '';
                container.style.opacity = '';
            }, 300);
        },

        toggleEditorMaximize() {
            this.isEditorMaximized = !this.isEditorMaximized;
            // 调整编辑器大小
            if (this.editor) {
                setTimeout(() => {
                    this.editor.layout();
                }, 300);
            }
        },

        // 用户菜单相关方法
        toggleUserMenu() {
            this.showUserMenu = !this.showUserMenu;
        },

        async changePassword() {
            if (!this.passwordForm.oldPassword || !this.passwordForm.newPassword || !this.passwordForm.confirmPassword) {
                alert('请填写所有密码字段');
                return;
            }

            if (this.passwordForm.newPassword !== this.passwordForm.confirmPassword) {
                alert('新密码和确认密码不匹配');
                return;
            }

            try {
                await this.request('/user/change-password', {
                    method: 'POST',
                    data: {
                        oldPassword: CryptoJS.MD5(this.passwordForm.oldPassword).toString(),
                        newPassword: CryptoJS.MD5(this.passwordForm.newPassword).toString()
                    }
                });

                alert('密码修改成功');
                this.showChangePassword = false;
                this.passwordForm = {
                    oldPassword: '',
                    newPassword: '',
                    confirmPassword: ''
                };
            } catch (error) {
                console.error('修改密码失败:', error);
                alert('修改密码失败: ' + (error.response?.data?.error || '未知错误'));
            }
        },

        // 点击其他区域关闭用户菜单
        handleClickOutside(event) {
            const userInfo = document.querySelector('.user-info');
            if (userInfo && !userInfo.contains(event.target)) {
                this.showUserMenu = false;
            }
        },

        // 收藏相关方法
        async loadFavorites() {
            try {
                const response = await this.request('/favorites');
                this.favorites = response || [];
            } catch (error) {
                console.error('加载收藏失败:', error);
                this.$toast('加载收藏失败');
            }
        },

        async saveFavorites() {
            try {
                await this.request('/favorites', {
                    method: 'POST',
                    data: this.favorites
                });
            } catch (error) {
                console.error('保存收藏失败:', error);
                this.$toast('保存收藏失败');
            }
        },

        toggleFavorite(file) {
            // 使用绝对路径，优先使用已保存的路径
            const absolutePath = file.absolutePath || file.path || (this.currentPath + (this.currentPath.endsWith('/') ? '' : '/') + file.name);
            const favoriteItem = {
                name: file.name,
                path: absolutePath,
                isDir: file.isDir,
                absolutePath: absolutePath  // 保存绝对路径
            };
            
            // 检查是否已经存在该收藏（使用路径进行比较）
            const existingIndex = this.favorites.findIndex(f => f.path === absolutePath || f.absolutePath === absolutePath);
            if (existingIndex !== -1) {
                // 如果已存在，则移除
                this.favorites.splice(existingIndex, 1);
                this.$toast('已取消收藏');
            } else {
                // 如果不存在，则添加
                this.favorites.push(favoriteItem);
                this.$toast('已添加到收藏');
            }
            
            // 保存到服务器
            this.saveFavorites();
        },

        isFavorite(file) {
            // 使用绝对路径进行比较，优先使用已保存的路径
            const absolutePath = file.absolutePath || file.path || (this.currentPath + (this.currentPath.endsWith('/') ? '' : '/') + file.name);
            return this.favorites.some(f => f.path === absolutePath || f.absolutePath === absolutePath);
        },

        handleFavoriteClick(fav) {
            // 如果是目录，则导航到该目录
            if (fav.isDir) {
                this.currentPath = fav.absolutePath || fav.path;
                this.listFiles();
            } else {
                // 如果是文件，则编辑该文件
                this.editFile({
                    name: fav.name,
                    path: fav.absolutePath || fav.path,
                    isDir: fav.isDir
                });
            }
        },

        $toast(message) {
            const toast = document.createElement('div');
            toast.className = 'toast';
            toast.textContent = message;
            document.body.appendChild(toast);
            
            // 添加显示类以触发动画
            setTimeout(() => toast.classList.add('show'), 10);
            
            // 3秒后移除
            setTimeout(() => {
                toast.classList.remove('show');
                setTimeout(() => document.body.removeChild(toast), 100);
            }, 1000);
        },

        // 添加 initEditor 方法
        initEditor() {
            // 等待Monaco Editor加载完成
            if (!window.monaco_ready) {
                setTimeout(() => this.initEditor(), 100);
                return;
            }

            const editorContainer = document.getElementById('modal-editor');
            if (!editorContainer) {
                console.error('Editor container not found');
                return;
            }

            // 如果编辑器已存在，先销毁
            if (this.editor) {
                this.editor.dispose();
                this.editor = null;
            }

            // 创建新的编辑器实例
            this.editor = monaco.editor.create(editorContainer, {
                value: '',
                theme: 'vs-dark',
                automaticLayout: true,
                minimap: { enabled: true },
                scrollBeyondLastLine: false,
                fontSize: 14,
                lineNumbers: 'on',
                renderWhitespace: 'selection',
                tabSize: 4,
                wordWrap: 'on',
                quickSuggestions: false,
                suggestOnTriggerCharacters: false,
                parameterHints: { enabled: false },
                suggest: { enabled: false }
            });

            // 确保编辑器在弹窗显示后调整大小
            setTimeout(() => {
                if (this.editor) {
                    this.editor.layout();
                    this.editor.focus();
                }
            }, 100);
        },

        // 显示权限设置弹窗
        showPermissionModal(file) {
            this.currentFile = file;
            // 解析当前权限字符串，确保permissions属性存在
            const permStr = file.permissions || '----------';
            
            // 跳过第一个字符（文件类型标识），只处理后面9个权限字符
            const permissionPart = permStr.length > 9 ? permStr.slice(-9) : permStr;
            
            const parsePermissions = (str) => ({
                read: str[0] === 'r',
                write: str[1] === 'w',
                execute: str[2] === 'x'
            });
            
            this.permissions = {
                owner: parsePermissions(permissionPart.slice(0, 3)),
                group: parsePermissions(permissionPart.slice(3, 6)),
                others: parsePermissions(permissionPart.slice(6, 9)),
                recursive: false
            };
            
            this.showPermissions = true;
        },
        
        // 保存权限设置
        async savePermissions() {
            if (!this.currentFile) return;
            
            try {
                // 将权限转换为八进制数字
                const calculateMode = (perm) => {
                    let mode = 0;
                    if (perm.read) mode += 4;
                    if (perm.write) mode += 2;
                    if (perm.execute) mode += 1;
                    return mode;
                };
                
                const ownerMode = calculateMode(this.permissions.owner);
                const groupMode = calculateMode(this.permissions.group);
                const othersMode = calculateMode(this.permissions.others);
                
                // 组合成完整的八进制权限数字
                const mode = (ownerMode * 64) + (groupMode * 8) + othersMode;
                
                await this.request('/files/chmod', {
                    method: 'POST',
                    data: {
                        path: this.currentPath + '/' + this.currentFile.name,
                        mode: mode.toString(),
                        recursive: this.permissions.recursive
                    }
                });
                
                this.showPermissions = false;
                await this.listFiles();
                this.$toast('权限修改成功');
            } catch (error) {
                console.error('修改权限失败:', error);
                this.$toast('修改权限失败: ' + (error.response?.data?.error || '未知错误'));
            }
        },
    },
    watch: {
        currentView(newView) {
            if (newView === 'system') this.getSystemInfo();
            if (newView === 'files') this.listFiles();
            if (newView === 'process') this.listProcesses();
            if (newView === 'terminal') {
                // 如果终端已经连接，重新适配终端大小并聚焦
                if (this.terminal && this.sshConnected) {
                    this.$nextTick(() => {
                        const terminalElement = document.getElementById('terminal');
                        terminalElement.style.display = 'block';
                        this.terminal.open(terminalElement);
                        if (this.fitAddon) {
                            this.fitAddon.fit();
                        }
                        // 自动聚焦到终端
                        this.terminal.focus();
                    });
                }
            }
        }
    },
    mounted() {
        const token = localStorage.getItem('token');
        if (token) {
            this.token = token;
            this.isLoggedIn = true;
            // 确保初始状态正确
            this.isEditing = false;
            this.currentEditingFile = null;
            this.$nextTick(async () => {
                await this.getSystemInfo();
                // 加载收藏列表
                await this.loadFavorites();
            });
        }

        // 添加点击事件监听器，用于关闭用户菜单
        document.addEventListener('click', this.handleClickOutside);
    },

    beforeDestroy() {
        // 移除点击事件监听器
        document.removeEventListener('click', this.handleClickOutside);
    }
}); 