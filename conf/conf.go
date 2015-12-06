package conf

var (
	//调用栈踪迹缓冲长度
	LenStackBuf = 4096

	//日志级别
	LogLevel string
	//日志路径
	LogPath string

	//控制台端口，默认不开启
	ConsolePort int
	//控制台提示符
	ConsolePrompt string = "squash# "
	//profile路径
	ProfilePath string

	//集群监听地址
	ListenAddr string
	//连接地址集合
	ConnAddrs []string
	//发送缓冲区长度
	PendingWriteNum int
)
