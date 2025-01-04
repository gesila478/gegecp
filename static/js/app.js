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
            },
            network: {
                sent: 0,
                recv: 0,
                sent_speed: 0,
                recv_speed: 0,
                trend: 0
            }
        },
        // 添加系统负载历史数据
        loadHistory: {
            cpu: [],
            memory: [],
            disk: [],
            network: [],
            timestamps: []
        },
        loadChart: null,
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
        showContextMenu: false,
        contextMenuStyle: {
            top: '0px',
            left: '0px'
        },
        selectedFile: null,
        isPathEditing: false,
        editingPath: '',
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
                disk: { total: 0, used: 0, free: 0, trend: 0 },
                network: { sent: 0, recv: 0, sent_speed: 0, recv_speed: 0, trend: 0 }
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
                const oldNetworkSpeed = (this.systemInfo.network.sent_speed + this.systemInfo.network.recv_speed) / (1024 * 1024) || 0;

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
                    },
                    network: {
                        sent: Number(response.network.sent || 0),
                        recv: Number(response.network.recv || 0),
                        sent_speed: Number(response.network.sent_speed || 0),
                        recv_speed: Number(response.network.recv_speed || 0),
                        trend: Number(((response.network.sent_speed + response.network.recv_speed) / (1024 * 1024)) - oldNetworkSpeed || 0)
                    }
                };

                // 更新历史数据
                if (response.history) {
                    this.loadHistory.timestamps = response.history.map(item => {
                        const date = new Date(item.Timestamp);
                        return date.getHours().toString().padStart(2, '0') + ':' +
                               date.getMinutes().toString().padStart(2, '0') + ':' +
                               date.getSeconds().toString().padStart(2, '0');
                    });
                    this.loadHistory.cpu = response.history.map(item => Number(item.CPU).toFixed(1));
                    this.loadHistory.memory = response.history.map(item => Number(item.Memory).toFixed(1));
                    this.loadHistory.disk = response.history.map(item => Number(item.Disk).toFixed(1));
                    this.loadHistory.network = response.history.map(item => Number(item.Network).toFixed(2));
                }

                // 更新图表
                this.updateLoadChart();

                // 更新进程列表
                this.processes = await this.request('/process/list');

                setTimeout(() => this.getSystemInfo(), 5000);
            } catch (error) {
                console.error('获取系统信息失败:', error);
            }
        },

        // 初始化负载图表
        initLoadChart() {
            if (!document.getElementById('loadChart')) return;
            
            this.loadChart = echarts.init(document.getElementById('loadChart'));
            const option = {
                title: {
                    textStyle: {
                        fontSize: 14,
                        fontWeight: 'normal'
                    }
                },
                tooltip: {
                    trigger: 'axis',
                    formatter: function(params) {
                        let result = params[0].axisValue + '<br/>';
                        params.forEach(param => {
                            let value = param.value;
                            if (param.seriesName === '网络使用率') {
                                value = value + ' MB/s';
                            } else {
                                value = value + '%';
                            }
                            result += param.seriesName + ': ' + value + '<br/>';
                        });
                        return result;
                    }
                },
                legend: {
                    data: ['CPU使用率', '内存使用率', '磁盘使用率', '网络使用率'],
                    bottom: 0
                },
                grid: {
                    left: '3%',
                    right: '4%',
                    bottom: '80px',
                    top: '30px',
                    containLabel: true
                },
                dataZoom: [
                    {
                        type: 'slider',
                        show: true,
                        bottom: 30,
                        height: 20,
                        start: 75,
                        end: 100,
                        handleSize: 20,
                        showDetail: true,
                        borderColor: '#ddd',
                        handleStyle: {
                            color: '#fff',
                            borderColor: '#ACB8D1'
                        },
                        textStyle: {
                            color: '#666'
                        },
                        backgroundColor: '#fff',
                        fillerColor: 'rgba(167,183,204,0.4)',
                        moveHandleSize: 5
                    },
                    {
                        type: 'inside',
                        start: 75,
                        end: 100,
                        zoomOnMouseWheel: true,
                        moveOnMouseMove: true
                    }
                ],
                xAxis: {
                    type: 'category',
                    boundaryGap: false,
                    data: this.loadHistory.timestamps,
                    axisLabel: {
                        fontSize: 10,
                        formatter: function(value) {
                            return value;
                        },
                        rotate: 45
                    }
                },
                yAxis: [
                    {
                        type: 'value',
                        name: '使用率',
                        min: 0,
                        max: 100,
                        splitLine: {
                            lineStyle: {
                                type: 'dashed'
                            }
                        },
                        axisLabel: {
                            formatter: '{value}%'
                        }
                    },
                    {
                        type: 'value',
                        name: '网络',
                        splitLine: {
                            show: false
                        },
                        axisLabel: {
                            formatter: '{value}MB/s'
                        }
                    }
                ],
                series: [
                    {
                        name: 'CPU使用率',
                        type: 'line',
                        smooth: true,
                        data: this.loadHistory.cpu,
                        itemStyle: {
                            color: '#409EFF'
                        },
                        showSymbol: false,
                        areaStyle: {
                            color: new echarts.graphic.LinearGradient(0, 0, 0, 1, [{
                                offset: 0,
                                color: 'rgba(64,158,255,0.3)'
                            }, {
                                offset: 1,
                                color: 'rgba(64,158,255,0.1)'
                            }])
                        }
                    },
                    {
                        name: '内存使用率',
                        type: 'line',
                        smooth: true,
                        data: this.loadHistory.memory,
                        itemStyle: {
                            color: '#67C23A'
                        },
                        showSymbol: false,
                        areaStyle: {
                            color: new echarts.graphic.LinearGradient(0, 0, 0, 1, [{
                                offset: 0,
                                color: 'rgba(103,194,58,0.3)'
                            }, {
                                offset: 1,
                                color: 'rgba(103,194,58,0.1)'
                            }])
                        }
                    },
                    {
                        name: '磁盘使用率',
                        type: 'line',
                        smooth: true,
                        data: this.loadHistory.disk,
                        itemStyle: {
                            color: '#E6A23C'
                        },
                        showSymbol: false,
                        areaStyle: {
                            color: new echarts.graphic.LinearGradient(0, 0, 0, 1, [{
                                offset: 0,
                                color: 'rgba(230,162,60,0.3)'
                            }, {
                                offset: 1,
                                color: 'rgba(230,162,60,0.1)'
                            }])
                        }
                    },
                    {
                        name: '网络使用率',
                        type: 'line',
                        smooth: true,
                        yAxisIndex: 1,
                        data: this.loadHistory.network,
                        itemStyle: {
                            color: '#F56C6C'
                        },
                        showSymbol: false,
                        areaStyle: {
                            color: new echarts.graphic.LinearGradient(0, 0, 0, 1, [{
                                offset: 0,
                                color: 'rgba(245,108,108,0.3)'
                            }, {
                                offset: 1,
                                color: 'rgba(245,108,108,0.1)'
                            }])
                        }
                    }
                ]
            };
            this.loadChart.setOption(option);

            // 监听窗口大小变化
            window.addEventListener('resize', () => {
                if (this.loadChart) {
                    this.loadChart.resize();
                }
            });
        },

        // 更新负载图表
        updateLoadChart() {
            if (!this.loadChart) {
                this.initLoadChart();
                return;
            }

            this.loadChart.setOption({
                xAxis: {
                    data: this.loadHistory.timestamps
                },
                series: [
                    {
                        name: 'CPU使用率',
                        data: this.loadHistory.cpu
                    },
                    {
                        name: '内存使用率',
                        data: this.loadHistory.memory
                    },
                    {
                        name: '磁盘使用率',
                        data: this.loadHistory.disk
                    },
                    {
                        name: '网络使用率',
                        data: this.loadHistory.network
                    }
                ]
            });
        },

        // 文件管理
        async listFiles() {
            try {
                this.files = await this.request('/files/list', {
                    params: { path: this.currentPath }
                });
            } catch (error) {
                console.error('获取文件列表失败:', error);
                // 添加友好的错误提示
                this.$toast(error.response?.status === 500 ? 
                    '访问路径不存在或无权限访问' : 
                    '获取文件列表失败: ' + (error.response?.data?.error || '未知错误'));
                
                // 如果路径不存在，返回上一级目录
                if (error.response?.status === 500) {
                    const parts = this.currentPath.split('/').filter(Boolean);
                    parts.pop();
                    this.currentPath = parts.length === 0 ? '/' : '/' + parts.join('/');
                    // 重新加载文件列表
                    if (this.currentPath !== '/opt/gegecp') {
                        this.listFiles();
                    }
                }
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

        formatSpeed(bytesPerSecond) {
            if (!bytesPerSecond && bytesPerSecond !== 0) return '0 B/s';
            const k = 1024;
            const sizes = ['B/s', 'KB/s', 'MB/s', 'GB/s'];
            const i = Math.floor(Math.log(Math.abs(bytesPerSecond)) / Math.log(k));
            return parseFloat((bytesPerSecond / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i];
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

        // 右键菜单相关方法
        handleContextMenu(event, file) {
            event.preventDefault();
            this.selectedFile = file;
            this.showContextMenu = true;
            this.contextMenuStyle = {
                top: `${event.clientY}px`,
                left: `${event.clientX}px`
            };
        },

        handleMenuClick(action) {
            if (!this.selectedFile) return;
            
            switch (action) {
                case 'open':
                    this.handleFileClick(this.selectedFile);
                    break;
                case 'edit':
                    this.editFile(this.selectedFile);
                    break;
                case 'download':
                    this.downloadFile(this.selectedFile);
                    break;
                case 'delete':
                    this.deleteFile(this.selectedFile);
                    break;
                case 'chmod':
                    this.showPermissionModal(this.selectedFile);
                    break;
            }
            this.showContextMenu = false;
        },

        // 点击其他地方关闭右键菜单
        closeContextMenu() {
            this.showContextMenu = false;
        },

        // 路径导航相关方法
        startPathEdit() {
            this.isPathEditing = true;
            this.editingPath = this.currentPath;
            this.$nextTick(() => {
                const pathInput = document.querySelector('.path-input');
                if (pathInput) {
                    pathInput.focus();
                    pathInput.select();
                }
            });
        },

        handlePathInputKeydown(event) {
            if (event.key === 'Enter') {
                event.preventDefault();
                this.submitPathEdit();
            } else if (event.key === 'Escape') {
                event.preventDefault();
                this.cancelPathEdit();
            }
        },

        submitPathEdit() {
            let path = this.editingPath.trim();
            // 确保路径以 / 开头
            if (!path.startsWith('/')) {
                path = '/' + path;
            }
            // 移除结尾的 / (除非是根路径)
            if (path !== '/' && path.endsWith('/')) {
                path = path.slice(0, -1);
            }
            this.currentPath = path;
            this.isPathEditing = false;
            this.listFiles();
        },

        cancelPathEdit() {
            this.isPathEditing = false;
            this.editingPath = this.currentPath;
        },
    },
    watch: {
        currentView(newView) {
            if (newView === 'system') {
                this.getSystemInfo();
                this.$nextTick(() => {
                    this.initLoadChart();
                    // 监听窗口大小变化
                    window.addEventListener('resize', () => {
                        if (this.loadChart) {
                            this.loadChart.resize();
                        }
                    });
                });
            }
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

        // 添加点击事件监听器，用于关闭用户菜单和右键菜单
        document.addEventListener('click', (event) => {
            // 关闭用户菜单
            const userInfo = document.querySelector('.user-info');
            if (userInfo && !userInfo.contains(event.target)) {
                this.showUserMenu = false;
            }
            
            // 关闭右键菜单
            const contextMenu = document.querySelector('.context-menu');
            if (contextMenu && !contextMenu.contains(event.target)) {
                this.showContextMenu = false;
            }
        });
    },

    beforeDestroy() {
        // 移除点击事件监听器
        document.removeEventListener('click', this.handleClickOutside);
    }
}); 