package tui

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

var sqlKeywordFormat = regexp.MustCompile(`(?i)\b(select|from|where|join|left join|right join|inner join|outer join|group by|order by|having|limit|offset|union|insert into|update|delete from|values|set)\b`)
var sqlPosition = regexp.MustCompile(`(?i)(?:position|character)\s+(\d+)`)

func formatSQL(sql string) string {
	statement := strings.TrimSpace(sql)
	if statement == "" {
		return ""
	}
	var formatted strings.Builder
	start := 0
	for index := 0; index < len(statement); {
		if end, ok := sqlQuotedEnd(statement, index); ok {
			formatted.WriteString(formatSQLCode(statement[start:index]))
			formatted.WriteString(statement[index:end])
			start, index = end, end
			continue
		}
		index++
	}
	formatted.WriteString(formatSQLCode(statement[start:]))
	return strings.TrimSpace(formatted.String())
}

func formatSQLCode(code string) string {
	formatted := sqlKeywordFormat.ReplaceAllStringFunc(code, strings.ToUpper)
	formatted = regexp.MustCompile(`\s+`).ReplaceAllString(formatted, " ")
	for _, keyword := range []string{"LEFT JOIN", "RIGHT JOIN", "INNER JOIN", "OUTER JOIN", "GROUP BY", "ORDER BY", "DELETE FROM", "INSERT INTO", "FROM", "WHERE", "JOIN", "HAVING", "LIMIT", "OFFSET", "UNION", "VALUES", "SET"} {
		formatted = strings.ReplaceAll(formatted, " "+keyword+" ", "\n"+keyword+" ")
	}
	return formatted
}

// sqlQuotedEnd returns the end of a SQL string or quoted identifier beginning
// at index. Keeping these spans verbatim prevents formatting from changing
// user data or identifier spelling.
func sqlQuotedEnd(sql string, index int) (int, bool) {
	if index >= len(sql) || !strings.ContainsRune("'\"`", rune(sql[index])) {
		return 0, false
	}
	quote := sql[index]
	for cursor := index + 1; cursor < len(sql); cursor++ {
		if sql[cursor] != quote {
			continue
		}
		if cursor+1 < len(sql) && sql[cursor+1] == quote {
			cursor++
			continue
		}
		return cursor + 1, true
	}
	return len(sql), true
}

func (m Model) explainSQL() (Model, tea.Cmd) {
	sql := strings.TrimSpace(m.sqlInput.Value())
	if !isStreamableQuery(sql) {
		m.status = "EXPLAIN is available only for SELECT queries"
		return m, nil
	}
	if strings.Contains(strings.TrimSuffix(sql, ";"), ";") {
		m.status = "EXPLAIN accepts one SELECT statement"
		return m, nil
	}
	m, cmd := m.runSQL("EXPLAIN " + strings.TrimSuffix(sql, ";"))
	m.queryPreserveEditor = true
	return m, cmd
}

func sqlErrorStatus(sql string, err error) string {
	if err == nil {
		return ""
	}
	match := sqlPosition.FindStringSubmatch(err.Error())
	if len(match) != 2 {
		return "query failed: " + err.Error()
	}
	position, parseErr := strconv.Atoi(match[1])
	if parseErr != nil || position < 1 {
		return "query failed: " + err.Error()
	}
	line, column := 1, 1
	for index, r := range []rune(sql) {
		if index >= position-1 {
			break
		}
		if r == '\n' {
			line, column = line+1, 1
		} else {
			column++
		}
	}
	return fmt.Sprintf("query failed at line %d, column %d: %v", line, column, err)
}
