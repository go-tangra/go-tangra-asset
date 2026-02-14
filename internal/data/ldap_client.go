package data

import (
	"context"
	"crypto/tls"
	"fmt"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-ldap/ldap/v3"
	"github.com/tx7do/kratos-bootstrap/bootstrap"
)

// LdapEmployee represents an employee fetched from LDAP
type LdapEmployee struct {
	FirstName      string
	LastName       string
	Email          string
	Phone          string
	Department     string
	JobTitle       string
	EmployeeNumber string
	DN             string
}

// LdapClient handles LDAP connections and queries
type LdapClient struct {
	host            string
	port            string
	useTLS          bool
	skipTLSVerify   bool
	bindDN          string
	bindPassword    string
	baseDN          string
	searchFilter    string
	attrFirstName   string
	attrLastName    string
	attrEmail       string
	attrPhone       string
	attrDepartment  string
	attrJobTitle    string
	attrEmployeeNum string
	log             *log.Helper
}

// NewLdapClient creates a new LDAP client from environment variables
func NewLdapClient(ctx *bootstrap.Context) *LdapClient {
	return &LdapClient{
		host:            getEnvOrDefault("ASSET_LDAP_HOST", ""),
		port:            getEnvOrDefault("ASSET_LDAP_PORT", "389"),
		useTLS:          getEnvOrDefault("ASSET_LDAP_USE_TLS", "false") == "true",
		skipTLSVerify:   getEnvOrDefault("ASSET_LDAP_SKIP_TLS_VERIFY", "false") == "true",
		bindDN:          getEnvOrDefault("ASSET_LDAP_BIND_DN", ""),
		bindPassword:    getEnvOrDefault("ASSET_LDAP_BIND_PASSWORD", ""),
		baseDN:          getEnvOrDefault("ASSET_LDAP_BASE_DN", ""),
		searchFilter:    getEnvOrDefault("ASSET_LDAP_SEARCH_FILTER", "(objectClass=inetOrgPerson)"),
		attrFirstName:   getEnvOrDefault("ASSET_LDAP_ATTR_FIRST_NAME", "givenName"),
		attrLastName:    getEnvOrDefault("ASSET_LDAP_ATTR_LAST_NAME", "sn"),
		attrEmail:       getEnvOrDefault("ASSET_LDAP_ATTR_EMAIL", "mail"),
		attrPhone:       getEnvOrDefault("ASSET_LDAP_ATTR_PHONE", "telephoneNumber"),
		attrDepartment:  getEnvOrDefault("ASSET_LDAP_ATTR_DEPARTMENT", "department"),
		attrJobTitle:    getEnvOrDefault("ASSET_LDAP_ATTR_JOB_TITLE", "title"),
		attrEmployeeNum: getEnvOrDefault("ASSET_LDAP_ATTR_EMPLOYEE_NUMBER", "employeeNumber"),
		log:             ctx.NewLoggerHelper("ldap/data/asset-service"),
	}
}

// IsConfigured returns true if LDAP host and base DN are set
func (c *LdapClient) IsConfigured() bool {
	return c.host != "" && c.baseDN != ""
}

// FetchEmployees connects to LDAP, searches for users, and returns mapped results
func (c *LdapClient) FetchEmployees(_ context.Context) ([]LdapEmployee, error) {
	if !c.IsConfigured() {
		return nil, fmt.Errorf("LDAP is not configured")
	}

	scheme := "ldap"
	if c.useTLS {
		scheme = "ldaps"
	}
	url := fmt.Sprintf("%s://%s:%s", scheme, c.host, c.port)

	var conn *ldap.Conn
	var err error

	if c.useTLS {
		conn, err = ldap.DialURL(url, ldap.DialWithTLSConfig(&tls.Config{
			InsecureSkipVerify: c.skipTLSVerify, //nolint:gosec
		}))
	} else {
		conn, err = ldap.DialURL(url)
	}
	if err != nil {
		c.log.Errorf("failed to connect to LDAP: %v", err)
		return nil, fmt.Errorf("failed to connect to LDAP server: %w", err)
	}
	defer conn.Close()

	if c.bindDN != "" {
		err = conn.Bind(c.bindDN, c.bindPassword)
		if err != nil {
			c.log.Errorf("failed to bind to LDAP: %v", err)
			return nil, fmt.Errorf("failed to bind to LDAP: %w", err)
		}
	}

	attributes := []string{
		"dn",
		c.attrFirstName,
		c.attrLastName,
		c.attrEmail,
		c.attrPhone,
		c.attrDepartment,
		c.attrJobTitle,
		c.attrEmployeeNum,
	}

	searchReq := ldap.NewSearchRequest(
		c.baseDN,
		ldap.ScopeWholeSubtree,
		ldap.NeverDerefAliases,
		0,    // no size limit
		0,    // no time limit
		false,
		c.searchFilter,
		attributes,
		nil,
	)

	result, err := conn.SearchWithPaging(searchReq, 500)
	if err != nil {
		c.log.Errorf("LDAP search failed: %v", err)
		return nil, fmt.Errorf("LDAP search failed: %w", err)
	}

	employees := make([]LdapEmployee, 0, len(result.Entries))
	for _, entry := range result.Entries {
		emp := LdapEmployee{
			FirstName:      entry.GetAttributeValue(c.attrFirstName),
			LastName:        entry.GetAttributeValue(c.attrLastName),
			Email:           entry.GetAttributeValue(c.attrEmail),
			Phone:           entry.GetAttributeValue(c.attrPhone),
			Department:      entry.GetAttributeValue(c.attrDepartment),
			JobTitle:        entry.GetAttributeValue(c.attrJobTitle),
			EmployeeNumber:  entry.GetAttributeValue(c.attrEmployeeNum),
			DN:              entry.DN,
		}

		// Skip entries without a name
		if emp.FirstName == "" && emp.LastName == "" {
			c.log.Warnf("skipping LDAP entry with no name: %s", entry.DN)
			continue
		}

		employees = append(employees, emp)
	}

	c.log.Infof("fetched %d employees from LDAP", len(employees))
	return employees, nil
}
