package utils

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"strings"
)

func WriteSliceToFile(filename string, data []string) error {
	content := strings.Join(data, "\n")
	return ioutil.WriteFile(filename, []byte(content), 0644)
}

func ReadSliceFromFile(filename string) ([]string, error) {
	b, err := os.ReadFile(filename)
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil
		}
		return nil, err
	}
	lines := strings.Split(string(b), "\n")
	var out []string
	for _, l := range lines {
		l = strings.TrimSpace(l)
		if l != "" {
			out = append(out, l)
		}
	}
	return out, nil
}

func WriteJSONToFile(filename string, v interface{}) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return os.WriteFile(filename, b, 0o644)
}

func ReadJSONFromFile(filename string, v interface{}) error {
	b, err := os.ReadFile(filename)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if len(b) == 0 {
		return nil
	}
	return json.Unmarshal(b, v)
}
