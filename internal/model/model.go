package model

type RecordCategory string

const (
	CategoryNote     RecordCategory = "NOTE"
	CategoryHost     RecordCategory = "HOST"
	CategoryDatabase RecordCategory = "DATABASE"
)

type AuthType string

const (
	AuthNone     AuthType = "NONE"
	AuthPassword AuthType = "PASSWORD"
	AuthSSHKey   AuthType = "SSH_KEY"
)

type DBType string

const (
	DBMySQL      DBType = "MYSQL"
	DBPostgreSQL DBType = "POSTGRESQL"
)

type Record struct {
	ID        string         `json:"id"`
	Name      string         `json:"name"`
	Alias     string         `json:"alias"`
	Category  RecordCategory `json:"category"`
	Notes     string         `json:"notes"`
	Favorite  bool           `json:"favorite"`
	Deleted   string         `json:"-"`
	CreatedAt string         `json:"createdAt"`
	UpdatedAt string         `json:"updatedAt"`
}

type Credential struct {
	ID          string   `json:"id,omitempty"`
	RecordID    string   `json:"recordId,omitempty"`
	Label       string   `json:"label,omitempty"`
	Username    string   `json:"username"`
	AuthType    AuthType `json:"authType"`
	SecretValue string   `json:"secretValue,omitempty"`
	KeyPath     string   `json:"keyPath,omitempty"`
	Deleted     string   `json:"-"`
	CreatedAt   string   `json:"createdAt,omitempty"`
	UpdatedAt   string   `json:"updatedAt,omitempty"`
}

type SSHConnection struct {
	RecordID     string `json:"recordId,omitempty"`
	Host         string `json:"host"`
	Port         int    `json:"port"`
	CredentialID string `json:"credentialId,omitempty"`
	RouteID      string `json:"routeId,omitempty"`
	CreatedAt    string `json:"createdAt,omitempty"`
	UpdatedAt    string `json:"updatedAt,omitempty"`
}

type SSHRoute struct {
	ID          string     `json:"id"`
	Name        string     `json:"name"`
	Description string     `json:"description"`
	Hops        []RouteHop `json:"hops"`
	Deleted     string     `json:"-"`
	CreatedAt   string     `json:"createdAt"`
	UpdatedAt   string     `json:"updatedAt"`
}

type RouteHop struct {
	Seq          int    `json:"seq"`
	HostRecordID string `json:"hostRecordId"`
}

type DBConnection struct {
	RecordID     string `json:"recordId,omitempty"`
	DBType       DBType `json:"dbType"`
	Host         string `json:"host"`
	Port         int    `json:"port"`
	DatabaseName string `json:"databaseName,omitempty"`
	CredentialID string `json:"credentialId,omitempty"`
	CreatedAt    string `json:"createdAt,omitempty"`
	UpdatedAt    string `json:"updatedAt,omitempty"`
}

// RecordDetail is the aggregate used by CLI and HTTP. The schema supports
// multiple credentials; Phase 0-4 aggregate writes maintain one default item.
type RecordDetail struct {
	Record      Record         `json:"record"`
	Credentials []Credential   `json:"credentials,omitempty"`
	Credential  *Credential    `json:"credential,omitempty"`
	SSH         *SSHConnection `json:"ssh,omitempty"`
	Database    *DBConnection  `json:"database,omitempty"`
}

type RecordInput struct {
	Name       string         `json:"name"`
	Alias      string         `json:"alias"`
	Category   RecordCategory `json:"category"`
	Notes      string         `json:"notes"`
	Favorite   bool           `json:"favorite"`
	Credential *Credential    `json:"credential,omitempty"`
	SSH        *SSHConnection `json:"ssh,omitempty"`
	Database   *DBConnection  `json:"database,omitempty"`
}

type RouteInput struct {
	Name        string     `json:"name"`
	Description string     `json:"description"`
	Hops        []RouteHop `json:"hops"`
}
