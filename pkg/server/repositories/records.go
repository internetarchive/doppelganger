package repositories

import (
	"fmt"

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

func AddRecord(r *models.Record) error {
	q := scyllaSession.Query(
		scyllaTable.InsertBuilder().
			Unique().
			ToCql()).
		BindStruct(r)
	if err := q.ExecRelease(); err != nil {
		return err
	}

	return nil
}
