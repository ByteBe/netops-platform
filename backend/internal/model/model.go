// Package model 统一数据模型（GORM）
package model

import "time"

// User 用户（登录账号）
type User struct {
	ID             uint       `gorm:"primaryKey" json:"id"`
	Username       string     `gorm:"size:64;uniqueIndex;not null" json:"username"`
	EmployeeNo     string     `gorm:"size:64;index" json:"employee_no"` // 工号
	Email          string     `gorm:"size:128;index" json:"email"`
	PasswordHash   string     `gorm:"size:128" json:"-"`
	Salt           string     `gorm:"size:32" json:"-"`
	Role           string     `gorm:"size:16;default:operator" json:"role"` // admin/operator/viewer
	Status         string     `gorm:"size:16;default:active" json:"status"` // active/disabled
	MustChangePwd    bool       `gorm:"default:false" json:"must_change_pwd"`
	LastPwdChangeAt *time.Time `json:"last_pwd_change_at"`
	LastLoginAt     *time.Time `json:"last_login_at"`
	LastLoginIP     string     `gorm:"size:64" json:"last_login_ip"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

// AuditLog 操作审计日志
type AuditLog struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	UserID    uint      `gorm:"index" json:"user_id"`
	Username  string    `gorm:"size:64" json:"username"`
	Action    string    `gorm:"size:128" json:"action"`
	Method    string    `gorm:"size:8" json:"method"`
	Path      string    `gorm:"size:256" json:"path"`
	IP        string    `gorm:"size:64" json:"ip"`
	Detail    string    `gorm:"size:1024" json:"detail"`
	CreatedAt time.Time `gorm:"index" json:"created_at"`
}

// DeviceGroup 资源分组（设置资源管理：将监控设备分类管理）
type DeviceGroup struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Name      string    `gorm:"size:64;not null" json:"name"`
	Type      string    `gorm:"size:32;default:other" json:"type"` // server/switch/router/firewall/database/other
	ParentID  uint      `gorm:"default:0" json:"parent_id"`
	Color     string    `gorm:"size:16" json:"color"`
	Remark    string    `gorm:"size:256" json:"remark"`
	CreatedAt time.Time `json:"created_at"`
}

// MonitorDevice 纳管监控设备（SNMP）
type MonitorDevice struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	GroupID     uint      `gorm:"index;default:0" json:"group_id"`
	Name        string    `gorm:"size:128;not null" json:"name"`
	IP          string    `gorm:"size:64;index;not null" json:"ip"`
	Type        string    `gorm:"size:32;default:server" json:"type"` // server/switch/router/firewall/other
	Vendor      string    `gorm:"size:64" json:"vendor"`
	SNMPVersion string    `gorm:"size:8;default:2c" json:"snmp_version"` // 1/2c/3
	Community   string    `gorm:"size:128" json:"community"`
	Username    string    `gorm:"size:64" json:"username"` // v3
	AuthProto   string    `gorm:"size:16;default:md5" json:"auth_proto"`
	PrivProto   string    `gorm:"size:16;default:des" json:"priv_proto"`
	AuthPass    string    `gorm:"size:128" json:"auth_pass"`
	PrivPass    string    `gorm:"size:128" json:"priv_pass"`
	Port        int       `gorm:"default:161" json:"port"`
	Interval    int       `gorm:"default:60" json:"interval"` // 采集周期秒
	Enable      bool      `gorm:"default:false" json:"enable"`
	Status      string    `gorm:"size:16;default:unknown" json:"status"` // up/down/unknown
	NodeUUID    string    `gorm:"size:64;index;default:''" json:"node_uuid"` // 归属节点（下级上报的设备填下级UUID，本机空=本地）
	Remark      string    `gorm:"size:256" json:"remark"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// LinkTask 链路检测任务
type LinkTask struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Name      string    `gorm:"size:128;not null" json:"name"`
	Target    string    `gorm:"size:128;not null;index" json:"target"` // IP 或域名
	Method    string    `gorm:"size:16;default:cmd" json:"method"`     // icmp/tcp/cmd
	Port      int       `gorm:"default:0" json:"port"`                 // tcp 方法端口
	Interval  int       `gorm:"default:10" json:"interval"`            // 秒
	Timeout   int       `gorm:"default:5" json:"timeout"`              // 秒
	GroupID   uint      `gorm:"index;default:0" json:"group_id"`
	Color     string    `gorm:"size:16;default:#409EFF" json:"color"` // 曲线颜色
	Enabled   bool      `gorm:"default:false" json:"enabled"`
	Status    string    `gorm:"size:16;default:unknown" json:"status"` // up/down/unknown
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// TopoDevice 拓扑设备
type TopoDevice struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Name      string    `gorm:"size:128;not null" json:"name"`
	IP        string    `gorm:"size:64" json:"ip"`
	Type      string    `gorm:"size:32;default:switch" json:"type"` // switch/router/server/firewall
	Icon      string    `gorm:"size:64" json:"icon"`
	X         float64   `json:"x"`
	Y         float64   `json:"y"`
	Z         float64   `json:"z"`
	GroupID   uint      `gorm:"index;default:0" json:"group_id"`
	Remark    string    `gorm:"size:256" json:"remark"`
	CreatedAt time.Time `json:"created_at"`
}

// TopoLink 拓扑连线（支持多IP：一条线可关联多个IP地址，显示在连线上）
type TopoLink struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Name      string    `gorm:"size:128" json:"name"`
	SourceID  uint      `gorm:"index;not null" json:"source_id"`
	TargetID  uint      `gorm:"index;not null" json:"target_id"`
	Color     string    `gorm:"size:16;default:#67C23A" json:"color"`
	Bandwidth string    `gorm:"size:64" json:"bandwidth"`
	Status    string    `gorm:"size:16;default:unknown" json:"status"` // up/down
	Remark    string    `gorm:"size:256" json:"remark"`
	CreatedAt time.Time `json:"created_at"`
}

// TopoLinkIP 连线关联IP
type TopoLinkIP struct {
	ID     uint   `gorm:"primaryKey" json:"id"`
	LinkID uint   `gorm:"index" json:"link_id"`
	IP     string `gorm:"size:64" json:"ip"`
}

// DBInstance 数据库监控实例
type DBInstance struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Name      string    `gorm:"size:128;not null" json:"name"`
	Type      string    `gorm:"size:32;not null" json:"type"` // mysql/oracle/dm/kingbase/sqlite
	Host      string    `gorm:"size:128" json:"host"`
	Port      int       `json:"port"`
	User      string    `gorm:"size:64" json:"user"`
	Password  string    `gorm:"size:256" json:"password"`
	DBName    string    `gorm:"size:128" json:"db_name"`
	Interval  int       `gorm:"default:60" json:"interval"`
	Enable    bool      `gorm:"default:false" json:"enable"`
	Status    string    `gorm:"size:16;default:unknown" json:"status"`
	GroupID   uint      `gorm:"index;default:0" json:"group_id"`
	Remark    string    `gorm:"size:256" json:"remark"`
	CreatedAt time.Time `json:"created_at"`
}

// ScriptTemplate 脚本模板（模板全部在代码中制作，启动时写入数据库，前后端不可配置）
type ScriptTemplate struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	Code        string    `gorm:"size:64;uniqueIndex" json:"code"` // 代码中的模板标识
	Vendor      string    `gorm:"size:16;index" json:"vendor"`     // huawei/h3c
	DeviceType  string    `gorm:"size:16;index" json:"device_type"` // router/switch/ac
	Category    string    `gorm:"size:32;index" json:"category"`    // vlan/acl/nat/...
	Name        string    `gorm:"size:128" json:"name"`
	Description string    `gorm:"size:512" json:"description"`
	SchemaJSON  string    `gorm:"type:text" json:"-"` // 参数表单定义
	Enabled     bool      `gorm:"default:false" json:"enabled"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// ScriptHistory 生成历史
type ScriptHistory struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	TemplateID  uint      `gorm:"index" json:"template_id"`
	TemplateName string   `gorm:"size:128" json:"template_name"`
	Vendor      string    `gorm:"size:16" json:"vendor"`
	DeviceType  string    `gorm:"size:16" json:"device_type"`
	ParamsJSON  string    `gorm:"type:text" json:"-"`
	Script      string    `gorm:"type:text" json:"script"`
	Creator     string    `gorm:"size:64" json:"creator"`
	CreatedAt   time.Time `json:"created_at"`
}

// Subnet IP地址段
type Subnet struct {
	ID             uint      `gorm:"primaryKey" json:"id"`
	Name           string    `gorm:"size:128;not null" json:"name"`
	CIDR           string    `gorm:"column:cidr;size:64;uniqueIndex;not null" json:"cidr"`
	Gateway        string    `gorm:"size:64" json:"gateway"`
	VLAN           int       `json:"vlan"`
	GroupID        uint      `gorm:"index;default:0" json:"group_id"`
	BindingEnabled bool      `gorm:"default:false" json:"binding_enabled"` // 地址绑定功能开关
	Description    string    `gorm:"size:256" json:"description"`
	CreatedAt      time.Time `json:"created_at"`
}

// IPRecord IP使用记录
type IPRecord struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	SubnetID    uint      `gorm:"index;not null" json:"subnet_id"`
	IP          string    `gorm:"size:64;index;not null" json:"ip"`
	OwnerName   string    `gorm:"size:64" json:"owner_name"` // 使用人员姓名
	MAC         string    `gorm:"size:32" json:"mac"`        // 电脑MAC地址
	Office      string    `gorm:"size:128" json:"office"`    // 办公室
	BindDeviceID uint     `gorm:"index;default:0" json:"bind_device_id"` // 下发配置的交换机
	BindStatus  string    `gorm:"size:16;default:none" json:"bind_status"` // none/ok/fail
	BindLog     string    `gorm:"size:512" json:"bind_log"`
	Remark      string    `gorm:"size:256" json:"remark"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// BindDevice 下发绑定命令的设备（SSH 登录，区别于 SNMP 监控）
type BindDevice struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Name      string    `gorm:"size:128;not null" json:"name"`
	IP        string    `gorm:"size:64;not null" json:"ip"`
	SSHUser   string    `gorm:"size:64" json:"ssh_user"`
	SSHPort   int       `gorm:"default:22" json:"ssh_port"`
	AuthType  string    `gorm:"size:16;default:password" json:"auth_type"` // password/key
	Credential string   `gorm:"size:512" json:"credential,omitempty"`
	Vendor    string    `gorm:"size:16;default:huawei" json:"vendor"` // huawei/h3c
	Enable    bool      `gorm:"default:false" json:"enable"`
	Remark    string    `gorm:"size:256" json:"remark"`
	CreatedAt time.Time `json:"created_at"`
}

// TrafficRule 流量监控规则（专线大小与接口带宽可不同、上下行速率可不同）
type TrafficRule struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	DeviceID    uint      `gorm:"index;not null" json:"device_id"`
	Interface   string    `gorm:"size:128;not null" json:"interface"` // 接口名 ifIndex 或名称
	DisplayName string    `gorm:"size:128" json:"display_name"`       // 显示名（专线名）
	LineRate    float64   `gorm:"default:1000" json:"line_rate"`      // 专线大小 Mbps
	UpRate      float64   `gorm:"default:1000" json:"up_rate"`        // 上行速率 Mbps
	DownRate    float64   `gorm:"default:1000" json:"down_rate"`      // 下行速率 Mbps
	Color       string    `gorm:"size:16;default:#409EFF" json:"color"`
	Enable      bool      `gorm:"default:false" json:"enable"`
	Remark      string    `gorm:"size:256" json:"remark"`
	CreatedAt   time.Time `json:"created_at"`
}

// AIConfig AI 接入配置（云端API + 本地Ollama + 主流大模型，多AI调度）
type AIConfig struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	Name        string    `gorm:"size:64;not null" json:"name"`
	Provider    string    `gorm:"size:32;not null" json:"provider"` // openai/ollama/azure/qwen/deepseek/...
	BaseURL     string    `gorm:"size:256;not null" json:"base_url"`
	APIKey      string    `gorm:"size:512" json:"-"`
	Model       string    `gorm:"size:128" json:"model"`
	Temperature float64   `gorm:"default:0.7" json:"temperature"`
	Priority    int       `gorm:"default:0" json:"priority"` // 调度优先级
	Enable      bool      `gorm:"default:false" json:"enable"`
	Remark      string    `gorm:"size:256" json:"remark"`
	CreatedAt   time.Time `json:"created_at"`
}

// MCPAgent MCP Agent 接入配置（服务器地址+接口形式接入主流 agent）
type MCPAgent struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Name      string    `gorm:"size:64;not null" json:"name"`
	ServerURL string    `gorm:"size:256;not null" json:"server_url"`
	Endpoint  string    `gorm:"size:256" json:"endpoint"`
	AuthType  string    `gorm:"size:16;default:none" json:"auth_type"` // none/bearer/basic
	AuthToken string    `gorm:"size:512" json:"-"`
	Enable    bool      `gorm:"default:false" json:"enable"`
	Remark    string    `gorm:"size:256" json:"remark"`
	CreatedAt time.Time `json:"created_at"`
}

// EmailConfig 邮箱配置（通过用户填写的邮箱发送告警与巡检报告）
type EmailConfig struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	Name         string    `gorm:"size:64;not null" json:"name"`
	SMTPHost     string    `gorm:"size:128;not null" json:"smtp_host"`
	SMTPPort     int       `gorm:"default:465" json:"smtp_port"`
	User         string    `gorm:"size:128;not null" json:"user"`
	Password     string    `gorm:"size:256" json:"-"`
	UseSSL       bool      `gorm:"default:false" json:"use_ssl"`
	Enable       bool      `gorm:"default:false" json:"enable"` // 是否启用
	DefaultTo    string    `gorm:"size:512" json:"default_to"`
	Remark       string    `gorm:"size:256" json:"remark"`
	CreatedAt    time.Time `json:"created_at"`
}

// ReportRecord 巡检报告记录
type ReportRecord struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	Title      string    `gorm:"size:256" json:"title"`
	StartAt    time.Time `json:"start_at"`
	EndAt      time.Time `json:"end_at"`
	Content    string    `gorm:"type:text" json:"content"` // HTML 正文
	Creator    string    `gorm:"size:64" json:"creator"`
	SendEmail  bool      `gorm:"default:false" json:"send_email"`
	EmailTo    string    `gorm:"size:512" json:"email_to"`
	CreatedAt  time.Time `json:"created_at"`
}

// SystemSetting 系统设置（键值）
type SystemSetting struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Key       string    `gorm:"size:128;uniqueIndex" json:"key"`
	Value     string    `gorm:"size:1024" json:"value"`
	Remark    string    `gorm:"size:256" json:"remark"`
	UpdatedAt time.Time `json:"updated_at"`
}

// DockerHost Docker监控主机
type DockerHost struct {
	ID      uint   `gorm:"primaryKey" json:"id"`
	Name    string `gorm:"size:128" json:"name"`
	Address string `gorm:"size:256" json:"address"`
	Enabled bool   `gorm:"default:false" json:"enabled"`
	Remark  string `gorm:"size:256" json:"remark"`
}

// K8sCluster K8s集群配置
type K8sCluster struct {
	ID        uint   `gorm:"primaryKey" json:"id"`
	Name      string `gorm:"size:128" json:"name"`
	APIServer string `gorm:"size:256" json:"api_server"`
	Token     string `gorm:"size:512" json:"token"`
	Enabled   bool   `gorm:"default:false" json:"enabled"`
	Remark    string `gorm:"size:256" json:"remark"`
}

// Node 分布式节点（总部-省-市-县级联）
type Node struct {
	ID          uint       `gorm:"primaryKey" json:"id"`
	NodeUUID    string     `gorm:"size:64;uniqueIndex;not null" json:"node_uuid"` // 本节点唯一标识
	Name        string     `gorm:"size:128;not null" json:"name"`                 // 节点名称（单位名）
	ParentUUID  string     `gorm:"size:64;index;default:''" json:"parent_uuid"`   // 父节点 UUID
	Level       int        `gorm:"default:1" json:"level"`                       // 1=总部 2=省 3=市 4=县
	Address     string     `gorm:"size:256" json:"address"`                       // 上级访问地址（下级填）
	Token       string     `gorm:"size:128" json:"-"`                             // 认证 token
	Status      string     `gorm:"size:16;default:"offline"" json:"status"`        // online/offline
	LastSeenAt  *time.Time `json:"last_seen_at"`
	ContainerCnt int       `gorm:"default:0" json:"container_cnt"`
	DeviceCnt   int        `gorm:"default:0" json:"device_cnt"`
	Remark      string     `gorm:"size:256" json:"remark"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}