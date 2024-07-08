package helm

import (
	"fmt"
	"strings"

	"github.com/ChristofferNissen/helmper/pkg/registry"
	"github.com/ChristofferNissen/helmper/pkg/util/ternary"
)

// traverse helm chart values to determine if condition is met
func ConditionMet(condition string, values map[string]any) bool {
	pos := values
	enabled := false
	for _, e := range strings.Split(condition, ".") {
		switch v := pos[e].(type) {
		case string:
			enabled = v == "true"
		case bool:
			enabled = v
		case map[string](any):
			pos = v
		case interface{}:
			pos = pos[e].(map[string]any)
		}
	}
	return enabled
}

// traverse helm chart values data structure
func findImageReferencesAcc(data map[string]any, values map[string]any, useCustomValues bool, acc string) map[*registry.Image][]string {
	res := make(map[*registry.Image][]string)

	i := registry.Image{}
	for k, v := range data {
		switch v := v.(type) {

		// yaml key-value pair value type
		case bool:
			switch k {
			case "useDigest":
				i.UseDigest = v
			}
		case string:
			found := true

			switch k {
			case "registry":
				switch useCustomValues {
				case true:
					s, ok := values[k].(string)
					if ok {
						i.Registry = s
					} else {
						i.Registry = v
					}
				case false:
					i.Registry = v
				}
			case "repository":
				switch useCustomValues {
				case true:
					s, ok := values[k].(string)
					if ok {
						i.Repository = s
					} else {
						i.Repository = v
					}
				case false:
					i.Repository = v
				}
			case "image":
				switch useCustomValues {
				case true:
					s, ok := values[k].(string)
					if ok {
						i.Repository = s
					} else {
						i.Repository = v
					}
				case false:
					i.Repository = v
				}
			case "tag":
				switch useCustomValues {
				case true:
					s, ok := values[k].(string)
					if ok {
						i.Tag = s
					} else {
						i.Tag = v
					}
				case false:
					i.Tag = v
				}
			case "digest":
				switch useCustomValues {
				case true:
					s, ok := values[k].(string)
					if ok {
						i.Digest = s
					} else {
						i.Digest = v
					}
				case false:
					i.Digest = v
				}
			case "sha":
				switch useCustomValues {
				case true:
					s, ok := values[k].(string)
					if ok {
						i.Digest = s
					} else {
						i.Digest = v
					}
				case false:
					i.Digest = v
				}
			default:
				found = false
			}

			if found {
				res[&i] = append(res[&i], fmt.Sprintf("%s.%s", acc, k))
			}

		// nested yaml object
		case map[string]any:
			// same path in yaml

			// Only parsed enabled sections
			enabled := true
			for k1, v1 := range v {
				if k1 == "enabled" {
					switch value := v1.(type) {
					case string:
						enabled = value == "true"
					case bool:
						enabled = ConditionMet(k1, values[k].(map[string]any))
					}
				}
			}

			// if enabled, parse nested section
			if enabled {
				path := ternary.Ternary(acc == "", k, fmt.Sprintf("%s.%s", acc, k))
				nestedRes := findImageReferencesAcc(v, values[k].(map[string]any), useCustomValues, path)
				for k, v := range nestedRes {
					res[k] = v
				}
			}
		}
	}

	return res
}

func findImageReferences(data map[string]any, values map[string]any, useCustomValues bool) map[*registry.Image][]string {
	return findImageReferencesAcc(data, values, useCustomValues, "")
}

// traverse helm chart values data structure
func replaceImageReferences(data map[string]any, registry string) {

	for k, v := range data {
		switch v := v.(type) {

		// yaml key-value pair value type
		case string:

			switch k {
			case "registry":
				registry, _ = strings.CutPrefix(registry, "oci://")
				registry, _ = strings.CutSuffix(registry, "/charts")
				data[k] = registry
			case "image":
				fallthrough
			case "repository":
				if v != "" {
					registry, _ = strings.CutPrefix(registry, "oci://")
					registry, _ = strings.CutSuffix(registry, "/charts")

					s := strings.Split(v, "/")

					if len(s) > 1 {
						s[0] = registry
					} else {
						s = append([]string{registry}, s...)
					}

					data[k] = strings.Join(s, "/")
				}
			}

		// nested yaml object
		case map[string]any:
			replaceImageReferences(data[k].(map[string]any), registry)
		}
	}
}
