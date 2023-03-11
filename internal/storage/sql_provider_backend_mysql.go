package storage

import (
	"crypto/tls"
	"fmt"
	"path"
	"time"

	"github.com/go-sql-driver/mysql"

	"github.com/authelia/authelia/v4/internal/configuration/schema"
)

// MySQLProvider is a MySQL provider.
type MySQLProvider struct {
	SQLProvider
}

// NewMySQLProvider a MySQL provider.
func NewMySQLProvider(config *schema.Configuration, tconfig *tls.Config) (provider *MySQLProvider) {
	provider = &MySQLProvider{
		SQLProvider: NewSQLProvider(config, providerMySQL, providerMySQL, dsnMySQL(config.Storage.MySQL, tconfig)),
	}

	// All providers have differing SELECT existing table statements.
	provider.sqlSelectExistingTables = queryMySQLSelectExistingTables

	// Specific alterations to this provider.
	provider.sqlFmtRenameTable = queryFmtMySQLRenameTable

	return provider
}

func dsnMySQL(config *schema.MySQLStorageConfiguration, tconfig *tls.Config) (dataSourceName string) {
	dsnConfig := mysql.NewConfig()

	switch {
	case path.IsAbs(config.Host):
		dsnConfig.Net = sqlNetworkTypeUnixSocket
		dsnConfig.Addr = config.Host
	case config.Port == 0:
		dsnConfig.Net = sqlNetworkTypeTCP
		dsnConfig.Addr = fmt.Sprintf("%s:%d", config.Host, 3306)
	default:
		dsnConfig.Net = sqlNetworkTypeTCP
		dsnConfig.Addr = fmt.Sprintf("%s:%d", config.Host, config.Port)
	}

	if tconfig != nil {
		_ = mysql.RegisterTLSConfig("storage", tconfig)

		dsnConfig.TLSConfig = "storage"
	}

	switch config.Port {
	case 0:
		dsnConfig.Addr = config.Host
	default:
		dsnConfig.Addr = fmt.Sprintf("%s:%d", config.Host, config.Port)
	}

	dsnConfig.DBName = config.Database
	dsnConfig.User = config.Username
	dsnConfig.Passwd = config.Password
	dsnConfig.Timeout = config.Timeout
	dsnConfig.MultiStatements = true
	dsnConfig.ParseTime = true
	dsnConfig.Loc = time.Local

	return dsnConfig.FormatDSN()
}
