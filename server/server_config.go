package server

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/kelseyhightower/envconfig"
)

type Config struct {
	// DBHost binds to 127.0.0.1 because mysql client will try to use a socket connection over localhost
	// but we are typically running mysql in docker, so not exposing the socket
	DBHost       string `default:"127.0.0.1" envconfig:"db_host"`
	DBUser       string `default:"testuser" envconfig:"db_user"`
	DBPass       string `default:"testuser" envconfig:"db_pass"`
	DBName       string `default:"helloworld" envconfig:"db_name"`
	DBPort       string `default:"3306" envconfig:"db_port"`
	DBCACertPath string `default:"/home/seth/ca-certificate.crt" evnconfig:"cert_path"`

	// Hostname binds to all interfaces so docker services can connect to this server outside of docker
	Hostname          string        `default:"0.0.0.0" envconfig:"hostname"`
	Port              int           `default:"16666" evnconfig:"port"`
	InternalPort      int           `default:"16667" envconfig:"internal_port"`
	EnableSocialLogin bool          `default:"false" envconfig:"enable_social_login"`
	ShouldSecure      bool          `default:"false" envconfig:"should_secure"`
	EnableDebug       bool          `default:"true" envconfig:"enable_debug"`
	TaskExpiration    time.Duration `default:"1m" envconfig:"task_expiration"`

	SGAPIKey string `default:"" envconfig:"sendgrid_apikey"`

	Version string
}

func NewConfigFromEnv() (Config, error) {
	var c Config
	err := envconfig.Process("helloworld", &c)
	if err != nil {
		return Config{}, err
	}

	return c, nil
}

func (c Config) String() string {
	return fmt.Sprintf(
		"DBUser: %s, DBPass: <hidden>, DBName: %s, DBHost: %s, DBPort: %v, DBCACertPath: %s, Addr: %s, Port: %d, Internal Port: %d, EnableSocialLogin: %t, SecureCookies: %t, EnableDebug: %t, Version: %s",
		c.DBUser, c.DBName, c.DBHost, c.DBPort, c.DBCACertPath, c.Hostname, c.Port, c.InternalPort, c.EnableSocialLogin, c.ShouldSecure, c.EnableDebug, c.Version,
	)
}

func (c Config) MarshalJSON() ([]byte, error) {
	type Alias Config // Avoid recursion by creating an alias for Config
	return json.Marshal(&struct {
		DBPass string `json:"db_pass,omitempty"`
		*Alias
	}{
		DBPass: "<hidden>", // Hide the DB password in JSON output
		Alias:  (*Alias)(&c),
	})
}
