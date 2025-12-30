package models

import "time"

// EntryNode 浠ｈ〃鍏ュ彛鏈嶅姟鍣紙娴峰鏈猴級
type EntryNode struct {
	ID           uint   `json:"id"`
	Name         string `json:"name"`
	IP           string `json:"ip"`
	Port         int    `json:"port"`           // 閫氬父涓?443 鎴?8443
	Domain       string `json:"domain"`         // 鐢ㄤ簬 TLS
	Certificate  string `json:"certificate"`    // 璇佷功鏂囦欢璺緞
	Key          string `json:"key"`            // 绉侀挜鏂囦欢璺緞
	Fallback     string `json:"fallback"`       // 鍥炶惤鍦板潃锛屼緥濡?"127.0.0.1:8080"
	TargetExitID uint   `json:"target_exit_id"` // 涓€閿腑杞槧灏勭殑钀藉湴鑺傜偣 ID
	Protocol     string `json:"protocol"`       // vless
	Security     string `json:"security"`       // xtls-vision

	// V2Board 鍚屾閰嶇疆
	V2boardURL    string `json:"v2board_url"`     // V2Board API 鍦板潃
	V2boardKey    string `json:"v2board_key"`     // 閫氳瀵嗛挜
	V2boardNodeID int    `json:"v2board_node_id"` // V2Board 姝ｅ紡鑺傜偣鐨?ID
	V2boardType   string `json:"v2board_type"`    // v2ray, shadowsocks, trojan

	CreatedAt time.Time `json:"created_at"`
}

// ExitNode 浠ｈ〃钀藉湴鏈嶅姟鍣紙灏忛浮锛?
type ExitNode struct {
	ID        uint      `json:"id"`
	Name      string    `json:"name"`
	Address   string    `json:"address"`
	Port      int       `json:"port"`
	Protocol  string    `json:"protocol"` // shadowsocks, vmess, vless
	Config    string    `json:"config"`   // 瀛樺偍鍏蜂綋鐨勫崗璁厤缃?(JSON string)
	CreatedAt time.Time `json:"created_at"`
}

// ForwardingRule 瀹氫箟浜嗚浆鍙戞槧灏勫叧绯?
type ForwardingRule struct {
	ID          uint   `json:"id"`
	UserID      string `json:"user_id"`    // 瀵瑰簲 VLESS 鐨?UUID
	UserEmail   string `json:"user_email"` // 瀵瑰簲 VLESS 鐨?Email锛岀敤浜庤瘑鍒祦閲?
	EntryNodeID uint   `json:"entry_node_id"`
	ExitNodeID  uint   `json:"exit_node_id"`
	Enabled     bool   `json:"enabled"`
}
