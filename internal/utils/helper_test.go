package utils

import (
	"database/sql/driver"
	"fmt"
	"strings"
	"testing"

	"github.com/exasol/exasol-driver-go/pkg/errors"

	"github.com/stretchr/testify/assert"
)

func TestNamedValuesToValues(t *testing.T) {
	namedValues := []driver.NamedValue{{Name: ""}, {Name: ""}}
	values, err := NamedValuesToValues(namedValues)
	assert.Equal(t, []driver.Value{driver.Value(nil), driver.Value(nil)}, values)
	assert.NoError(t, err)
}

func TestNamedValuesToValuesInvalidName(t *testing.T) {
	namedValues := []driver.NamedValue{{Name: "some name"}}
	values, err := NamedValuesToValues(namedValues)
	assert.Nil(t, values)
	assert.EqualError(t, err, "E-EGOD-7: named parameters not supported")
}

func TestIsImportQuery(t *testing.T) {
	assert.True(t, IsImportQuery("IMPORT into <targettable> from local CSV file '/path/to/filename.csv' <optional options>;\n"))
}

func TestGetFilePathNotFound(t *testing.T) {
	query := "SELECT * FROM table"
	_, err := GetFilePaths(query)
	assert.ErrorIs(t, err, errors.ErrInvalidImportQuery)
}

func TestOpenFileNotFound(t *testing.T) {
	_, err := OpenFile("./.does_not_exist")
	assert.EqualError(t, err, "E-EGOD-28: file './.does_not_exist' not found")
}

func TestOpenFile(t *testing.T) {
	file, err := OpenFile("../../testData/data.csv")
	assert.NoError(t, err)
	assert.NotNil(t, file)
}

func TestUpdateImportQuery(t *testing.T) {
	query := "IMPORT into table FROM LOCAL CSV file '/path/to/filename.csv'"
	newQuery := UpdateImportQuery(query, "127.0.0.1", 4333)
	assert.Equal(t, "IMPORT into table FROM CSV AT 'http://127.0.0.1:4333' FILE 'data.csv' ", newQuery)
}

func TestUpdateImportQueryMulti(t *testing.T) {
	query := "IMPORT into table FROM LOCAL CSV file '/path/to/filename.csv' file '/path/to/filename2.csv'"
	newQuery := UpdateImportQuery(query, "127.0.0.1", 4333)
	assert.Equal(t, "IMPORT into table FROM CSV AT 'http://127.0.0.1:4333' FILE 'data.csv' ", newQuery)
}

func TestUpdateImportQueryMulti2(t *testing.T) {
	query := "IMPORT INTO table_1 FROM LOCAL CSV USER 'agent_007' IDENTIFIED BY 'secret' FILE 'tab1_part1.csv' FILE 'tab1_part2.csv' COLUMN SEPARATOR = ';' SKIP = 5;"
	newQuery := UpdateImportQuery(query, "127.0.0.1", 4333)
	assert.Equal(t, "IMPORT INTO table_1 FROM CSV AT 'http://127.0.0.1:4333' USER 'agent_007' IDENTIFIED BY 'secret' FILE 'data.csv' COLUMN SEPARATOR = ';' SKIP = 5;", newQuery)
}

func TestGetFilePaths(t *testing.T) {
	quotes := []struct {
		name  string
		value string
	}{
		{name: "SingleQuote",
			value: "'"},
		{name: "DoubleQuote",
			value: `"`},
	}

	tests := []struct {
		name  string
		paths []string
	}{
		{name: "Single file", paths: []string{"/path/to/filename.csv"}},
		{name: "Multi file", paths: []string{"/path/to/filename.csv", "/path/to/filename2.csv"}},
		{name: "Relative paths", paths: []string{"./tab1_part1.csv", "./tab1_part2.csv"}},
		{name: "Windows paths", paths: []string{"C:\\Documents\\Newsletters\\Summer2018.csv", "\\Program Files\\Custom Utilities\\StringFinder.csv"}},
		{name: "Unix paths", paths: []string{"/Users/User/Documents/Data/test.csv"}},
	}

	for _, quote := range quotes {
		for _, tt := range tests {
			t.Run(fmt.Sprintf("%s %s", tt.name, quote.name), func(t *testing.T) {
				var preparedPaths []string
				for _, path := range tt.paths {
					preparedPaths = append(preparedPaths, fmt.Sprintf("%s%s%s", quote.value, path, quote.value))
				}

				foundPaths, err := GetFilePaths(fmt.Sprintf(`IMPORT INTO table_1 FROM CSV
       			AT 'http://192.168.1.1:8080/' USER 'agent_007' IDENTIFIED BY 'secret'
       			FILE %s 
       			COLUMN SEPARATOR = ';'
       			SKIP = 5;`, strings.Join(preparedPaths, " FILE ")))
				assert.NoError(t, err)
				assert.ElementsMatch(t, tt.paths, foundPaths)
			})
		}
	}
}

func TestGetRowSeparatorLF(t *testing.T) {
	query := "IMPORT into table FROM LOCAL CSV file '/path/to/filename.csv' ROW SEPARATOR = 'LF'"
	assert.Equal(t, GetRowSeparator(query), "\n")
}

func TestGetRowSeparatorCR(t *testing.T) {
	query := "IMPORT into table FROM LOCAL CSV file '/path/to/filename.csv' ROW SEPARATOR = 'CR'"
	assert.Equal(t, GetRowSeparator(query), "\r")
}

func TestGetRowSeparatorCRLF(t *testing.T) {
	query := "IMPORT into table FROM LOCAL CSV file '/path/to/filename.csv' ROW SEPARATOR =  'CRLF'"
	assert.Equal(t, GetRowSeparator(query), "\r\n")
}

func TestGetRowSeparator(t *testing.T) {
	tests := []struct {
		name      string
		separator string
		want      string
	}{
		{name: "LF", separator: "LF", want: "\n"},
		{name: "LF lowercase", separator: "lf", want: "\n"},
		{name: "CRLF", separator: "CRLF", want: "\r\n"},
		{name: "CRLF lowercase", separator: "crlf", want: "\r\n"},
		{name: "CR", separator: "CR", want: "\r"},
		{name: "CR lowercase", separator: "cr", want: "\r"},
	}
	for _, tt := range tests {
		query := fmt.Sprintf("IMPORT into table FROM LOCAL CSV file '/path/to/filename.csv' ROW SEPARATOR =  '%s'", tt.separator)

		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, GetRowSeparator(query))
		})
	}
}

func TestSingleHostResolve(t *testing.T) {
	hosts, err := ResolveHosts("localhost")

	assert.NoError(t, err)
	assert.Equal(t, 1, len(hosts))
	assert.Equal(t, "localhost", hosts[0])
}

func TestMultipleHostResolve(t *testing.T) {
	hosts, err := ResolveHosts("exasol1,127.0.0.1,exasol3")

	assert.NoError(t, err)
	assert.Equal(t, 3, len(hosts))
	assert.Equal(t, "exasol1", hosts[0])
	assert.Equal(t, "127.0.0.1", hosts[1])
	assert.Equal(t, "exasol3", hosts[2])
}

func TestHostSuffixRangeResolve(t *testing.T) {
	hosts, err := ResolveHosts("exasol1..3")

	assert.NoError(t, err)
	assert.Equal(t, 3, len(hosts))
	assert.Equal(t, "exasol1", hosts[0])
	assert.Equal(t, "exasol2", hosts[1])
	assert.Equal(t, "exasol3", hosts[2])
}

func TestResolvingHostRangeWithCompleteHostnameNotSupported(t *testing.T) {
	hosts, err := ResolveHosts("exasol1..exasol3")

	assert.NoError(t, err)
	assert.Equal(t, 1, len(hosts))
	assert.Equal(t, "exasol1..exasol3", hosts[0])
}

func TestResolvingHostRangeWithInvalidRangeNotSupported(t *testing.T) {
	hosts, err := ResolveHosts("exasolX..Y")

	assert.NoError(t, err)
	assert.Equal(t, 1, len(hosts))
	assert.Equal(t, "exasolX..Y", hosts[0])
}

func TestResolvingHostRangeWithInvalidRangeLimits(t *testing.T) {
	hosts, err := ResolveHosts("exasol3..1")
	assert.EqualError(t, err, "E-EGOD-20: invalid host range limits: 'exasol3..1'")
	assert.Nil(t, hosts)
}

func TestIPRangeResolve(t *testing.T) {
	hosts, err := ResolveHosts("127.0.0.1..3")
	assert.NoError(t, err)
	assert.Equal(t, 3, len(hosts))
	assert.Equal(t, "127.0.0.1", hosts[0])
	assert.Equal(t, "127.0.0.2", hosts[1])
	assert.Equal(t, "127.0.0.3", hosts[2])
}
