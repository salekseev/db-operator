package database

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/kloeckner-i/db-operator/pkg/utils/kci"

	// do not delete
	_ "github.com/go-sql-driver/mysql"

	"github.com/GoogleCloudPlatform/cloudsql-proxy/proxy/dialers/mysql"
	"github.com/sirupsen/logrus"
)

// Mysql is a database interface, abstraced object
// represents a database on mysql instance
// can be used to execute query to mysql database
type Mysql struct {
	Backend  string
	Host     string
	Port     int32
	Database string
	User     string
	Password string
}

// CheckStatus checks status of mysql database
// if the connection to database works
func (m Mysql) CheckStatus() error {
	db, err := m.getDbConn(m.User, m.Password)
	if err != nil {
		return err
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		db.Close()
		return fmt.Errorf("db conn test failed - could not establish a connection: %v", err)
	}

	check := fmt.Sprintf("USE %s", m.Database)
	if _, err := db.Exec(check); err != nil {
		return err
	}

	return nil
}

func (m Mysql) getDbConn(user, password string) (*sql.DB, error) {
	var db *sql.DB
	var err error

	switch m.Backend {
	case "google":
		db, err = mysql.DialPassword(m.Host, user, password)
		if err != nil {
			logrus.Debugf("failed to validate db connection: %s", err)
			return db, err
		}
	default:
		dataSourceName := fmt.Sprintf("%s:%s@tcp(%s:%d)/", user, password, m.Host, m.Port)
		db, err = sql.Open("mysql", dataSourceName)
		if err != nil {
			logrus.Debugf("failed to validate db connection: %s", err)
			return db, err
		}
		db.SetMaxIdleConns(0)
	}

	return db, nil
}

func (m Mysql) executeQuery(query string, admin AdminCredentials) error {
	db, err := m.getDbConn(admin.Username, admin.Password)
	if err != nil {
		logrus.Fatalf("failed to get db connection: %s", err)
	}

	defer db.Close()
	_, err = db.Query(query)
	if err != nil {
		logrus.Debugf("failed to execute query: %s", err)
		return err
	}

	return nil
}

func (m Mysql) createDatabase(admin AdminCredentials) error {
	create := fmt.Sprintf("CREATE DATABASE IF NOT EXISTS `%s`;", m.Database)

	err := m.executeQuery(create, admin)
	if err != nil {
		return err
	}

	return nil
}

func (m Mysql) deleteDatabase(admin AdminCredentials) error {
	create := fmt.Sprintf("DROP DATABASE IF EXISTS `%s`;", m.Database)

	err := kci.Retry(3, 5*time.Second, func() error {
		err := m.executeQuery(create, admin)
		if err != nil {
			logrus.Debugf("failed error: %s...retry...", err)
			return err
		}

		return nil
	})
	if err != nil {
		logrus.Debugf("retry failed  %s", err)
		return err
	}

	return nil
}

func (m Mysql) createUser(admin AdminCredentials) error {
	create := fmt.Sprintf("CREATE USER `%s` IDENTIFIED BY '%s';", m.User, m.Password)
	grant := fmt.Sprintf("GRANT ALL PRIVILEGES ON `%s`.* TO '%s'@'%%';", m.Database, m.User)
	update := fmt.Sprintf("SET PASSWORD FOR `%s` = PASSWORD('%s');", m.User, m.Password)

	if !m.isUserExist(admin) {
		err := m.executeQuery(create, admin)
		if err != nil {
			return err
		}
	} else {
		err := m.executeQuery(update, admin)
		if err != nil {
			return err
		}
	}

	err := m.executeQuery(grant, admin)
	if err != nil {
		return err
	}

	return nil
}

func (m Mysql) deleteUser(admin AdminCredentials) error {
	delete := fmt.Sprintf("DROP USER `%s`;", m.User)

	if m.isUserExist(admin) {
		err := m.executeQuery(delete, admin)
		if err != nil {
			return err
		}
	}

	return nil
}

func (m Mysql) isRowExist(query string, admin AdminCredentials) bool {
	db, err := m.getDbConn(admin.Username, admin.Password)
	if err != nil {
		logrus.Fatalf("failed to get db connection: %s", err)
	}
	defer db.Close()

	var result string
	err = db.QueryRow(query).Scan(&result)
	if err != nil {
		logrus.Debug(err)
		return false
	}

	return true
}

func (m Mysql) isUserExist(admin AdminCredentials) bool {
	check := fmt.Sprintf("SELECT User FROM mysql.user WHERE user='%s';", m.User)

	if m.isRowExist(check, admin) {
		logrus.Debug("user exists")
		return true
	}

	logrus.Debug("user doesn't exists")
	return false
}

// GetCredentials returns credentials of the mysql database
func (m Mysql) GetCredentials() Credentials {

	return Credentials{
		Name:     m.Database,
		Username: m.User,
		Password: m.Password,
	}
}