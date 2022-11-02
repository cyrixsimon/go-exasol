package exasol

import (
	"context"
	"database/sql/driver"
	"encoding/json"
)

type statement struct {
	connection      *connection
	statementHandle int
	columns         []sqlQueryColumn
	numInput        int
}

func (s *statement) QueryContext(ctx context.Context, args []driver.NamedValue) (driver.Rows, error) {
	values, err := namedValuesToValues(args)
	if err != nil {
		return nil, err
	}
	result, err := s.executePreparedStatement(ctx, values)
	if err != nil {
		return nil, err
	}
	return toRow(result, s.connection)
}

func (s *statement) Query(args []driver.Value) (driver.Rows, error) {
	result, err := s.executePreparedStatement(context.Background(), args)
	if err != nil {
		return nil, err
	}
	return toRow(result, s.connection)
}

func (s *statement) ExecContext(ctx context.Context, args []driver.NamedValue) (driver.Result, error) {
	values, err := namedValuesToValues(args)
	if err != nil {
		return nil, err
	}
	result, err := s.executePreparedStatement(ctx, values)
	if err != nil {
		return nil, err
	}
	return toResult(result)
}

func (s *statement) Exec(args []driver.Value) (driver.Result, error) {
	result, err := s.executePreparedStatement(context.Background(), args)
	if err != nil {
		return nil, err
	}
	return toResult(result)
}

func (s *statement) Close() error {
	if s.connection.isClosed {
		return driver.ErrBadConn
	}
	return s.connection.send(context.Background(), &closePreparedStatementCommand{
		command:         command{"closePreparedStatement"},
		StatementHandle: s.statementHandle,
	}, nil)
}

func (s *statement) NumInput() int {
	return s.numInput
}

func toResult(result *sqlQueriesResponse) (driver.Result, error) {
	rowCountResult := &sqlQueryResponseRowCount{}
	err := json.Unmarshal(result.Results[0], rowCountResult)
	if err != nil {
		return nil, err
	}

	return &rowCount{
		affectedRows: int64(rowCountResult.RowCount),
	}, err
}

func (s *statement) executePreparedStatement(ctx context.Context, args []driver.Value) (*sqlQueriesResponse, error) {
	columns := s.columns
	if len(args)%len(columns) != 0 {
		return nil, ErrInvalidValuesCount
	}

	data := make([][]interface{}, len(columns))
	for i, arg := range args {
		if data[i%len(columns)] == nil {
			data[i%len(columns)] = make([]interface{}, 0)
		}
		data[i%len(columns)] = append(data[i%len(columns)], arg)
	}

	command := &executePreparedStatementCommand{
		command:         command{"executePreparedStatement"},
		StatementHandle: s.statementHandle,
		Columns:         columns,
		NumColumns:      len(columns),
		NumRows:         len(data[0]),
		Data:            data,
		Attributes: attributes{
			ResultSetMaxRows: s.connection.config.resultSetMaxRows,
		},
	}
	result := &sqlQueriesResponse{}
	err := s.connection.send(ctx, command, result)
	if err != nil {
		return nil, err
	}
	if result.NumResults == 0 {
		return nil, ErrMalformedData
	}
	return result, err
}

func toRow(result *sqlQueriesResponse, con *connection) (driver.Rows, error) {
	resultSet := &sqlQueryResponseResultSet{}
	err := json.Unmarshal(result.Results[0], resultSet)
	if err != nil {
		return nil, err
	}

	return &queryResults{data: &resultSet.ResultSet, con: con}, err
}
