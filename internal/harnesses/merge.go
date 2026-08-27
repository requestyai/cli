package harnesses

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"github.com/pelletier/go-toml/v2"
	"gopkg.in/yaml.v3"
)

func mergeOrCreateJSONConfigFile(path string, patch map[string]any) (map[string]any, error) {
	return mergeJSONConfigFileWithOptions(path, patch, mergeOptions{
		AllowFileNotExists: true,
	})
}

func mergeJSONConfigFile(path string, patch map[string]any) (map[string]any, error) {
	return mergeJSONConfigFileWithOptions(path, patch, mergeOptions{
		AllowFileNotExists: false,
	})
}

type mergeOptions struct {
	AllowFileNotExists bool
}

func mergeJSONConfigFileWithOptions(path string, patch map[string]any, options mergeOptions) (map[string]any, error) {
	dataBytes, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) && options.AllowFileNotExists {
		dataBytes = []byte("{}\n")
	} else if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	// Use a decoder that uses json.Number to avoid
	// json.Unmarshal decoding ints into float64 when
	// unmarshalling to interface{}.
	decoder := json.NewDecoder(bytes.NewReader(dataBytes))
	decoder.UseNumber()

	data := make(map[string]any)
	if err := decoder.Decode(&data); err != nil {
		return nil, fmt.Errorf("failed to decode: %w", err)
	}

	if err := mergePatch(data, patch, ""); err != nil {
		return nil, fmt.Errorf("failed to merge: %w", err)
	}

	return data, nil
}

func mergeOrCreateTOMLConfigFile(path string, patch map[string]any) (map[string]any, error) {
	return mergeTOMLConfigFileWithOptions(path, patch, mergeOptions{
		AllowFileNotExists: true,
	})
}

func mergeTOMLConfigFile(path string, patch map[string]any) (map[string]any, error) {
	return mergeTOMLConfigFileWithOptions(path, patch, mergeOptions{
		AllowFileNotExists: false,
	})
}

func mergeTOMLConfigFileWithOptions(path string, patch map[string]any, options mergeOptions) (map[string]any, error) {
	dataBytes, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) && options.AllowFileNotExists {
		dataBytes = []byte{}
	} else if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	data := make(map[string]any)
	if err := toml.Unmarshal(dataBytes, &data); err != nil {
		return nil, fmt.Errorf("failed to unmarshal: %w", err)
	}

	if err := mergePatch(data, patch, ""); err != nil {
		return nil, fmt.Errorf("failed to merge: %w", err)
	}

	return data, nil
}

func mergeOrCreateYAMLConfigFile(path string, patch map[string]any) (map[string]any, error) {
	return mergeYAMLConfigFileWithOptions(path, patch, mergeOptions{
		AllowFileNotExists: true,
	})
}

func mergeYAMLConfigFileWithOptions(path string, patch map[string]any, options mergeOptions) (map[string]any, error) {
	dataBytes, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) && options.AllowFileNotExists {
		dataBytes = []byte("{}\n")
	} else if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	data := make(map[string]any)
	if err := yaml.Unmarshal(dataBytes, &data); err != nil {
		return nil, fmt.Errorf("failed to unmarshal: %w", err)
	}

	if err := mergePatch(data, patch, ""); err != nil {
		return nil, fmt.Errorf("failed to merge: %w", err)
	}

	return data, nil
}

// removal marks a patch key the merge should delete rather than set, which is how
// a value the CLI wrote on an earlier run is taken back out of a config.
type removal struct{}

// mergePatch recursively applies patch values while preserving unrelated fields.
// It rejects nested patches when the existing value is not an object or table.
func mergePatch(destination, patch map[string]any, path string) error {
	for key, patchValue := range patch {
		if _, isRemoval := patchValue.(removal); isRemoval {
			delete(destination, key)
			continue
		}

		patchMap, isMap := patchValue.(map[string]any)
		if !isMap {
			destination[key] = patchValue
			continue
		}

		fieldPath := key
		if path != "" {
			fieldPath = path + "." + key
		}

		destinationValue, exists := destination[key]
		if !exists || destinationValue == nil {
			destinationValue = make(map[string]any)
		}

		destinationMap, ok := destinationValue.(map[string]any)
		if !ok {
			return fmt.Errorf("expected field %q to be an object or table", fieldPath)
		}
		if err := mergePatch(destinationMap, patchMap, fieldPath); err != nil {
			return err
		}

		// A patch of nothing but removals leaves an object with nothing in it,
		// which says no more than having no object at all.
		if len(destinationMap) == 0 {
			delete(destination, key)
			continue
		}

		destination[key] = destinationMap
	}

	return nil
}
