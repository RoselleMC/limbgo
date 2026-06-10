package registrydata

import (
	_ "embed"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"sync"
)

//go:embed registrydata.json
var embeddedData []byte

type Registry struct {
	ID      string
	Entries []Entry
}

type TagRegistry struct {
	ID   string
	Tags []Tag
}

type Tag struct {
	Key    string
	Values []int32
}

type Entry struct {
	Key   string
	Value []byte
}

type Data struct {
	registries map[int32][]Registry
	tags       map[int32][]TagRegistry
	codecs     map[int32][]byte
	dimensions map[int32][]byte
}

type encodedData struct {
	Registries      map[string][]encodedRegistry    `json:"registries"`
	Tags            map[string][]encodedTagRegistry `json:"tags,omitempty"`
	DimensionCodecs map[string]string               `json:"dimension_codecs"`
	Dimensions      map[string]string               `json:"dimensions"`
}

type encodedRegistry struct {
	ID      string         `json:"id"`
	Entries []encodedEntry `json:"entries"`
}

type encodedEntry struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type encodedTagRegistry struct {
	ID   string       `json:"id"`
	Tags []encodedTag `json:"tags"`
}

type encodedTag struct {
	Key    string  `json:"key"`
	Values []int32 `json:"values"`
}

var (
	defaultOnce sync.Once
	defaultData *Data
	defaultErr  error
)

func Registries(protocol int32) ([]Registry, bool) {
	data, err := Default()
	if err != nil {
		return nil, false
	}
	registries, ok := data.Registries(protocol)
	return registries, ok
}

func Tags(protocol int32) ([]TagRegistry, bool) {
	data, err := Default()
	if err != nil {
		return nil, false
	}
	tags, ok := data.Tags(protocol)
	return tags, ok
}

func DimensionCodec(protocol int32) ([]byte, bool) {
	data, err := Default()
	if err != nil {
		return nil, false
	}
	codec, ok := data.DimensionCodec(protocol)
	return codec, ok
}

func Default() (*Data, error) {
	defaultOnce.Do(func() {
		defaultData, defaultErr = LoadBytes(embeddedData)
	})
	return defaultData, defaultErr
}

func LoadFile(path string) (*Data, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return LoadBytes(data)
}

func LoadBytes(raw []byte) (*Data, error) {
	var encoded encodedData
	if err := json.Unmarshal(raw, &encoded); err != nil {
		return nil, fmt.Errorf("parse registry data: %w", err)
	}
	out := &Data{
		registries: map[int32][]Registry{},
		tags:       map[int32][]TagRegistry{},
		codecs:     map[int32][]byte{},
		dimensions: map[int32][]byte{},
	}
	for rawProtocol, encodedRegistries := range encoded.Registries {
		protocol, err := parseProtocol(rawProtocol)
		if err != nil {
			return nil, err
		}
		for _, encodedRegistry := range encodedRegistries {
			registry := Registry{ID: encodedRegistry.ID}
			for _, encodedEntry := range encodedRegistry.Entries {
				value, err := base64.StdEncoding.DecodeString(encodedEntry.Value)
				if err != nil {
					return nil, fmt.Errorf("decode registry %d %s/%s: %w", protocol, encodedRegistry.ID, encodedEntry.Key, err)
				}
				registry.Entries = append(registry.Entries, Entry{
					Key:   encodedEntry.Key,
					Value: value,
				})
			}
			out.registries[protocol] = append(out.registries[protocol], registry)
		}
	}
	for rawProtocol, encodedTagRegistries := range encoded.Tags {
		protocol, err := parseProtocol(rawProtocol)
		if err != nil {
			return nil, err
		}
		for _, encodedTagRegistry := range encodedTagRegistries {
			tagRegistry := TagRegistry{ID: encodedTagRegistry.ID}
			for _, encodedTag := range encodedTagRegistry.Tags {
				values := append([]int32(nil), encodedTag.Values...)
				tagRegistry.Tags = append(tagRegistry.Tags, Tag{
					Key:    encodedTag.Key,
					Values: values,
				})
			}
			out.tags[protocol] = append(out.tags[protocol], tagRegistry)
		}
	}
	for rawProtocol, rawCodec := range encoded.DimensionCodecs {
		protocol, err := parseProtocol(rawProtocol)
		if err != nil {
			return nil, err
		}
		codec, err := base64.StdEncoding.DecodeString(rawCodec)
		if err != nil {
			return nil, fmt.Errorf("decode dimension codec %d: %w", protocol, err)
		}
		out.codecs[protocol] = codec
	}
	for rawProtocol, rawDimension := range encoded.Dimensions {
		protocol, err := parseProtocol(rawProtocol)
		if err != nil {
			return nil, err
		}
		dimension, err := base64.StdEncoding.DecodeString(rawDimension)
		if err != nil {
			return nil, fmt.Errorf("decode dimension %d: %w", protocol, err)
		}
		out.dimensions[protocol] = dimension
	}
	return out, nil
}

func (d *Data) Registries(protocol int32) ([]Registry, bool) {
	registries, ok := d.registries[protocol]
	return registries, ok
}

func (d *Data) Tags(protocol int32) ([]TagRegistry, bool) {
	tags, ok := d.tags[protocol]
	return tags, ok
}

func (d *Data) DimensionCodec(protocol int32) ([]byte, bool) {
	codec, ok := d.codecs[protocol]
	return codec, ok
}

func (d *Data) Dimension(protocol int32) ([]byte, bool) {
	dimension, ok := d.dimensions[protocol]
	return dimension, ok
}

func parseProtocol(value string) (int32, error) {
	protocol, err := strconv.ParseInt(value, 10, 32)
	if err != nil {
		return 0, fmt.Errorf("parse protocol %q: %w", value, err)
	}
	return int32(protocol), nil
}
