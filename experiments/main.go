package experiments

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/Ultramarine-Linux/um/pkg/util"
	"github.com/samber/lo"
	bolt "go.etcd.io/bbolt"
)

type StabilityLevel int

const (
	GFL StabilityLevel = iota
	Devel
	Alpha
	Beta
)

func (s StabilityLevel) String() string {
	switch s {
	case GFL:
		return "GFL"
	case Devel:
		return "Devel"
	case Alpha:
		return "Alpha"
	case Beta:
		return "Beta"
	default:
		return "Unknown"
	}
}

type Experiment struct {
	Id          string         `json:"id"`
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Stability   StabilityLevel `json:"stability"`
	UpScript    string         `json:"-"`
	DownScript  string         `json:"-"`
	Enabled     bool           `json:"-"`
}

func List() ([]Experiment, error) {
	db, err := util.GetDB()
	if err != nil {
		return nil, err
	}
	defer db.Close()

	experimentsManifest := filepath.Join(util.GetDataDir(), "experiments.json")

	file, err := os.ReadFile(experimentsManifest)
	if err != nil {
		return nil, err
	}

	var experiments []Experiment

	if err := json.Unmarshal(file, &experiments); err != nil {
		return nil, err
	}

	return lo.Map(experiments, func(exp Experiment, index int) Experiment {
		exp.UpScript = filepath.Join(util.GetDataDir(), "experiments", exp.Id, "up")
		exp.DownScript = filepath.Join(util.GetDataDir(), "experiments", exp.Id, "down")
		db.View(func(tx *bolt.Tx) error {
			bucket := tx.Bucket([]byte("experiments"))
			if bucket == nil {
				return nil
			}

			if bucket.Get([]byte(exp.Id))[0] == 1 {
				exp.Enabled = true
			}

			return nil
		})
		return exp
	}), nil
}

func Find(id string) (*Experiment, error) {
	exps, err := List()
	if err != nil {
		return nil, err
	}

	exp, found := lo.Find(exps, func(item Experiment) bool {
		return item.Id == id
	})

	if !found {
		return nil, nil
	}

	return &exp, nil
}

func MarkEnabled(id string, status bool) error {
	db, err := util.GetDB()
	if err != nil {
		return err
	}
	defer db.Close()

	db.Update(func(tx *bolt.Tx) error {
		bucket, err := tx.CreateBucketIfNotExists([]byte("experiments"))
		if err != nil {
			return err
		}

		var statusByte byte
		if status {
			statusByte = 1
		} else {
			statusByte = 0
		}

		if err := bucket.Put([]byte(id), []byte{statusByte}); err != nil {
			return err
		}

		return nil
	})

	return nil
}
