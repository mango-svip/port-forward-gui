package main

import (
    "fmt"
    "fyne.io/fyne/v2"
    "fyne.io/fyne/v2/app"
    "fyne.io/fyne/v2/container"
    "fyne.io/fyne/v2/dialog"
    "fyne.io/fyne/v2/widget"
    "io"
    "net"
    "sync"
    "sync/atomic"
    "time"
)

// ForwardConfig 存储转发配置
type ForwardConfig struct {
    LocalPort  string
    RemoteHost string
    RemotePort string
    Active     bool

    listener    net.Listener
    Connections atomic.Int64
    conns       map[net.Conn]struct{}
    mutex       sync.Mutex
}

// ForwardManager 管理所有转发配置
type ForwardManager struct {
    configs []*ForwardConfig
    mutex   sync.RWMutex
    table   *widget.Table
}

func NewForwardManager() *ForwardManager {
    return &ForwardManager{
        configs: make([]*ForwardConfig, 0),
    }
}

// 启动端口转发
func (f *ForwardConfig) startForward() error {
    if f.listener != nil {
        return fmt.Errorf("服务已经在运行中")
    }

    listener, err := net.Listen("tcp", fmt.Sprintf(":%s", f.LocalPort))
    if err != nil {
        return err
    }
    f.listener = listener
    f.conns = make(map[net.Conn]struct{})

    go func() {
        for {
            conn, err := listener.Accept()
            if err != nil {
                if !f.Active {
                    return
                }
                break
            }
            f.Connections.Add(1)
            f.mutex.Lock()
            f.conns[conn] = struct{}{}
            f.mutex.Unlock()
            go f.handleConnection(conn)
        }
    }()

    return nil
}

func (f *ForwardConfig) stopForward() {
    if f.listener != nil {
        f.Active = false
        f.listener.Close()
        f.listener = nil
        f.mutex.Lock()
        for c := range f.conns {
            c.Close()
        }
        f.conns = nil
        f.mutex.Unlock()
    }
}

// 处理单个连接的转发
func (f *ForwardConfig) handleConnection(clientConn net.Conn) {
    defer clientConn.Close()
    defer f.Connections.Add(-1)
    f.mutex.Lock()
    if f.conns != nil {
        delete(f.conns, clientConn)
    }
    f.mutex.Unlock()

    remoteConn, err := net.Dial("tcp", fmt.Sprintf("%s:%s", f.RemoteHost, f.RemotePort))
    if err != nil {
        fmt.Printf("无法连接到远程服务器 %s:%s: %v\n", f.RemoteHost, f.RemotePort, err)
        return
    }
    var _close = func() {
        clientConn.Close()
        remoteConn.Close()
    }
    defer _close()
    var wg sync.WaitGroup
    wg.Add(2)
    errChan := make(chan error, 2)

    // 客户端到远程服务器的数据转发
    go func() {
        defer wg.Done()
        _, err := io.Copy(remoteConn, clientConn)
        if err != nil && err != io.EOF {
            _close()
        }
        _close()
    }()

    // 远程服务器到客户端的数据转发
    go func() {
        defer wg.Done()
        _, err := io.Copy(clientConn, remoteConn)
        if err != nil && err != io.EOF {

        }
        _close()
    }()

    // 等待所有数据传输完成
    go func() {
        wg.Wait()
        close(errChan)
    }()

    // 处理错误
    for err := range errChan {
        fmt.Printf("连接错误: %v\n", err)
    }
}

func main() {
    manager := NewForwardManager()
    myApp := app.New()
    myWindow := myApp.NewWindow("端口转发工具")
    myWindow.Resize(fyne.NewSize(800, 600))

    // 创建添加新转发配置的表单
    localPortEntry := widget.NewEntry()
    localPortEntry.SetPlaceHolder("本地端口")

    remoteHostEntry := widget.NewEntry()
    remoteHostEntry.SetPlaceHolder("远程主机")

    remotePortEntry := widget.NewEntry()
    remotePortEntry.SetPlaceHolder("远程端口")

    addButton := widget.NewButton("添加", func() {
        config := &ForwardConfig{
            LocalPort:  localPortEntry.Text,
            RemoteHost: remoteHostEntry.Text,
            RemotePort: remotePortEntry.Text,
            Active:     false,
        }

        // 启动端口转发
        if err := config.startForward(); err != nil {
            dialog.ShowError(err, myWindow)
            return
        }
        config.Active = true

        manager.mutex.Lock()
        manager.configs = append(manager.configs, config)
        manager.mutex.Unlock()

        // 清空输入
        localPortEntry.SetText("")
        remoteHostEntry.SetText("")
        remotePortEntry.SetText("")
    })

    // 创建表格显示所有转发配置
    // list := binding.NewUntypedList()
    manager.table = widget.NewTable(
        func() (int, int) {
            manager.mutex.RLock()
            defer manager.mutex.RUnlock()
            return len(manager.configs), 5
        },
        func() fyne.CanvasObject {
            return container.NewHBox(widget.NewLabel(""))
        },
        func(i widget.TableCellID, o fyne.CanvasObject) {
            manager.mutex.RLock()
            config := manager.configs[i.Row]
            manager.mutex.RUnlock()

            container := o.(*fyne.Container)
            if i.Col < 3 {
                label := container.Objects[0].(*widget.Label)
                switch i.Col {
                case 0:
                    label.SetText(config.LocalPort)
                case 1:
                    label.SetText(config.RemoteHost + ":" + config.RemotePort)
                case 2:
                    label.SetText(fmt.Sprintf("%d", config.Connections.Load()))
                }
            } else {
                container.Objects = nil
                switch i.Col {
                case 3:
                    stopBtn := widget.NewButton("停止", func() {
                        manager.mutex.Lock()
                        config.stopForward()
                        manager.mutex.Unlock()
                        manager.table.Refresh()
                    })
                    if !config.Active {
                        stopBtn.Disable()
                    }
                    container.Add(stopBtn)
                case 4:
                    restartBtn := widget.NewButton("重启", func() {
                        manager.mutex.Lock()
                        if config.Active {
                            config.stopForward()
                        }
                        err := config.startForward()
                        if err != nil {
                            dialog.ShowError(err, myWindow)
                        } else {
                            config.Active = true
                        }
                        manager.mutex.Unlock()
                        manager.table.Refresh()
                    })
                    container.Add(restartBtn)
                }
            }
        },
    )

    // 设置表格列标题
    manager.table.SetColumnWidth(0, 100) // 本地端口
    manager.table.SetColumnWidth(1, 200) // 远程地址
    manager.table.SetColumnWidth(2, 100) // 连接数
    manager.table.SetColumnWidth(3, 80)  // 停止按钮
    manager.table.SetColumnWidth(4, 80)  // 重启按钮

    // 更新UI显示
    go func() {
        for {
            time.Sleep(time.Second)
            fyne.Do(manager.table.Refresh)
        }
    }()

    // 创建主布局
    form := container.NewGridWithColumns(4,
        localPortEntry,
        remoteHostEntry,
        remotePortEntry,
        addButton,
    )

    content := container.NewBorder(
        form,
        nil,
        nil,
        nil,
        manager.table,
    )

    myWindow.SetContent(content)
    myWindow.Show()
    myApp.Run()
}
