package audit

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

func LoadDataset(path string) (*Dataset, error) {
	body, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read dataset: %w", err)
	}

	var dataset Dataset
	if err := json.Unmarshal(body, &dataset); err != nil {
		return nil, fmt.Errorf("decode dataset: %w", err)
	}
	for index := range dataset.Cases {
		for snapshotIndex := range dataset.Cases[index].Snapshots {
			if dataset.Cases[index].Snapshots[snapshotIndex].File == "" {
				return nil, fmt.Errorf("case %s snapshot %d missing file", dataset.Cases[index].ID, snapshotIndex)
			}
			dataset.Cases[index].Snapshots[snapshotIndex].File = filepath.Clean(filepath.Join(filepath.Dir(path), dataset.Cases[index].Snapshots[snapshotIndex].File))
		}
	}
	return &dataset, nil
}
