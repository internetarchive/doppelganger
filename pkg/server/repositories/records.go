package repositories

import (
	"fmt"

	"github.com/gocql/gocql"
	"github.com/internetarchive/doppelganger/pkg/server/models"
	"github.com/scylladb/gocqlx/v3/qb"
)

var ErrRecordNotFound = fmt.Errorf("no record found")

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
			fmt.Sprintf("INSERT INTO %s (id, uri, date) VALUES (?, ?, ?)", scyllaTable.Name()),
			record.ID,
			record.URI,
			record.Date,
		)
	}

	if err := scyllaSession.ExecuteBatch(batch); err != nil {
		return err
	}

	return nil
}
