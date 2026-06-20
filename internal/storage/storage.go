package storage

import (
	"encoding/json"
	"fmt"
	"os"
)

type ResumeState struct {
	CompletedPieces []int `json:"completed_pieces"`
}

func SaveFile(path string, length int64) (*os.File, error) {
	f, err := os.Create(path)
	if err != nil {
		return nil, fmt.Errorf("storage: creating file: %v", err)
	}

	err = f.Truncate(length)
	if err != nil {
		f.Close()
		os.Remove(path)
		return nil, fmt.Errorf("storage: truncating file: %v", err)
	}

	return f, nil
}

func OpenFile(path string) (*os.File, error) {
	f, err := os.OpenFile(path, os.O_RDWR, 0644)
	if err != nil {
		return nil, fmt.Errorf("storage: opening file: %v", err)
	}
	return f, nil
}

func SaveResumeState(statePath string, completed []int) error {
	state := ResumeState{CompletedPieces: completed}
	data, err := json.Marshal(state)
	if err != nil {
		return fmt.Errorf("storage: marshaling state: %v", err)
	}

	err = os.WriteFile(statePath, data, 0644)
	if err != nil {
		return fmt.Errorf("storage: writing state file: %v", err)
	}

	return nil
}

func LoadResumeState(statePath string) ([]int, error) {
	data, err := os.ReadFile(statePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("storage: reading state file: %v", err)
	}

	var state ResumeState
	err = json.Unmarshal(data, &state)
	if err != nil {
		return nil, fmt.Errorf("storage: unmarshaling state: %v", err)
	}

	return state.CompletedPieces, nil
}

func FileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
