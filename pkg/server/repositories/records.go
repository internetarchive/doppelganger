package repositories

import (
	"fmt"

	"github.com/gocql/gocql"
	"github.com/internetarchive/doppelganger/pkg/server/models"
	"github.com/scylladb/gocqlx/v3/qb"
)

var (
	ErrRecordNotFound        = fmt.Errorf("no record found")
	ErrPendingRecordNotFound = fmt.Errorf("no pending record found")
)

func GetRecord(ID string) (*models.Record, error) {
	records := new([]*models.Record)
	q := scyllaSession.Query(scyllaTable.SelectBuilder().Where(qb.Eq("id")).Limit(1).ToCql()).BindMap(qb.M{"id": ID})
	if err := q.SelectRelease(records); err != nil {
		return nil, err
	}

	if len(*records) == 0 {
		return nil, ErrRecordNotFound
	}

	return (*records)[0], nil
}

func AddRecords(records ...*models.Record) error {
	batch := scyllaSession.NewBatch(gocql.LoggedBatch)

	for _, record := range records {
		batch.Query(
			fmt.Sprintf("INSERT INTO %s (id, uri, date, sha1, size) VALUES (?, ?, ?, ?, ?)", scyllaTable.Name()),
			record.ID,
			record.URI,
			record.Date,
			record.SHA1,
			record.Size,
		)
	}

	if err := scyllaSession.ExecuteBatch(batch); err != nil {
		return err
	}

	return nil
}

// AddPendingRecord adds a record to the pending table
func AddPendingRecord(record *models.Record) error {
	return scyllaSession.Query(
		fmt.Sprintf("INSERT INTO %s (id, uri, date, sha1, size) VALUES (?, ?, ?, ?, ?)", pendingTable.Name()),
		[]string{},
	).Bind(
		record.ID,
		record.URI,
		record.Date,
		record.SHA1,
		record.Size,
	).Exec()
}

// GetPendingRecord retrieves a record from the pending table
func GetPendingRecord(ID string) (*models.Record, error) {
	pendingRecords := new([]*models.Record)
	q := scyllaSession.Query(pendingTable.SelectBuilder().Where(qb.Eq("id")).Limit(1).ToCql()).BindMap(qb.M{"id": ID})
	if err := q.SelectRelease(pendingRecords); err != nil {
		return nil, err
	}

	if len(*pendingRecords) == 0 {
		return nil, ErrPendingRecordNotFound
	}

	return (*pendingRecords)[0], nil
}

// GetAllPendingRecords retrieves up to 1,000 records from the pending table to avoid overwhelming the system
func GetAllPendingRecords() ([]*models.Record, error) {
	var pendingRecords []*models.Record
	q := scyllaSession.Query(pendingTable.SelectBuilder().Limit(1000).ToCql())
	if err := q.SelectRelease(&pendingRecords); err != nil {
		return nil, err
	}

	return pendingRecords, nil
}

// MovePendingToRecords moves a record from pending to records table
func MovePendingToRecords(ID string, record *models.Record) error {
	batch := scyllaSession.NewBatch(gocql.LoggedBatch)

	// Insert into records table
	batch.Query(
		fmt.Sprintf("INSERT INTO %s (id, uri, date, sha1, size) VALUES (?, ?, ?, ?, ?)", scyllaTable.Name()),
		record.ID,
		record.URI,
		record.Date,
		record.SHA1,
		record.Size,
	)

	// Delete from pending table
	batch.Query(
		fmt.Sprintf("DELETE FROM %s WHERE id = ?", pendingTable.Name()),
		ID,
	)

	return scyllaSession.ExecuteBatch(batch)
}

// DeletePendingRecord removes a record from the pending table
func DeletePendingRecord(ID string) error {
	return scyllaSession.Query(
		fmt.Sprintf("DELETE FROM %s WHERE id = ?", pendingTable.Name()),
		[]string{},
	).Bind(ID).Exec()
}
