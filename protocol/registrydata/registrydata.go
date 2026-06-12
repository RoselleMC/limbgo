package registrydata

import (
	"archive/zip"
	"bytes"
	_ "embed"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
)

//go:embed registrydata.zip
var embeddedZip []byte

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

// Source returns a registry data snapshot for a new connection.
type Source interface {
	RegistryData() (*Data, error)
}

type encodedData struct {
	Registries      map[string][]encodedRegistry    `json:"registries"`
	Tags            map[string][]encodedTagRegistry `json:"tags,omitempty"`
	DimensionCodecs map[string]string               `json:"dimension_codecs"`
	Dimensions      map[string]string               `json:"dimensions"`
}

type encodedProtocolData struct {
	FormatVersion  int                  `json:"format_version,omitempty"`
	Protocol       int32                `json:"protocol,omitempty"`
	Registries     []encodedRegistry    `json:"registries,omitempty"`
	Tags           []encodedTagRegistry `json:"tags,omitempty"`
	DimensionCodec string               `json:"dimension_codec,omitempty"`
	Dimension      string               `json:"dimension,omitempty"`
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
		defaultData, defaultErr = LoadZipBytes(embeddedZip)
	})
	return defaultData, defaultErr
}

func (d *Data) RegistryData() (*Data, error) {
	if d == nil {
		return nil, fmt.Errorf("registry data is nil")
	}
	return d, nil
}

func LoadFile(path string) (*Data, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if isZip(data) {
		return LoadZipBytes(data)
	}
	return LoadBytes(data)
}

func LoadZipFile(path string) (*Data, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return LoadZipBytes(data)
}

func LoadBytes(raw []byte) (*Data, error) {
	var encoded encodedData
	if err := json.Unmarshal(raw, &encoded); err != nil {
		return nil, fmt.Errorf("parse registry data: %w", err)
	}
	out := newData()
	for rawProtocol, encodedRegistries := range encoded.Registries {
		protocol, err := parseProtocol(rawProtocol)
		if err != nil {
			return nil, err
		}
		if err := out.decodeRegistries(protocol, encodedRegistries); err != nil {
			return nil, err
		}
	}
	for rawProtocol, encodedTagRegistries := range encoded.Tags {
		protocol, err := parseProtocol(rawProtocol)
		if err != nil {
			return nil, err
		}
		out.decodeTags(protocol, encodedTagRegistries)
	}
	for rawProtocol, rawCodec := range encoded.DimensionCodecs {
		protocol, err := parseProtocol(rawProtocol)
		if err != nil {
			return nil, err
		}
		if err := out.decodeDimensionCodec(protocol, rawCodec); err != nil {
			return nil, err
		}
	}
	for rawProtocol, rawDimension := range encoded.Dimensions {
		protocol, err := parseProtocol(rawProtocol)
		if err != nil {
			return nil, err
		}
		if err := out.decodeDimension(protocol, rawDimension); err != nil {
			return nil, err
		}
	}
	return out, nil
}

func LoadZipBytes(raw []byte) (*Data, error) {
	reader, err := zip.NewReader(bytes.NewReader(raw), int64(len(raw)))
	if err != nil {
		return nil, fmt.Errorf("open registry data zip: %w", err)
	}
	out := newData()
	loaded := 0
	for _, file := range reader.File {
		if file.FileInfo().IsDir() || filepath.Ext(file.Name) != ".json" {
			continue
		}
		protocol, err := parseProtocol(protocolName(file.Name))
		if err != nil {
			return nil, fmt.Errorf("parse registry zip entry %q: %w", file.Name, err)
		}
		if err := out.loadZipEntry(protocol, file); err != nil {
			return nil, fmt.Errorf("load registry zip entry %q: %w", file.Name, err)
		}
		loaded++
	}
	if loaded == 0 {
		return nil, fmt.Errorf("registry data zip contains no protocol json files")
	}
	return out, nil
}

func newData() *Data {
	return &Data{
		registries: map[int32][]Registry{},
		tags:       map[int32][]TagRegistry{},
		codecs:     map[int32][]byte{},
		dimensions: map[int32][]byte{},
	}
}

func (d *Data) loadZipEntry(protocol int32, file *zip.File) error {
	handle, err := file.Open()
	if err != nil {
		return err
	}
	defer handle.Close()
	raw, err := io.ReadAll(handle)
	if err != nil {
		return err
	}
	var encoded encodedProtocolData
	if err := json.Unmarshal(raw, &encoded); err != nil {
		return fmt.Errorf("parse protocol %d registry data: %w", protocol, err)
	}
	if encoded.FormatVersion != 0 && encoded.FormatVersion != 1 {
		return fmt.Errorf("unsupported protocol %d registry data format %d", protocol, encoded.FormatVersion)
	}
	if encoded.Protocol != 0 && encoded.Protocol != protocol {
		return fmt.Errorf("protocol file declared protocol %d, want %d", encoded.Protocol, protocol)
	}
	if err := d.decodeRegistries(protocol, encoded.Registries); err != nil {
		return err
	}
	d.decodeTags(protocol, encoded.Tags)
	if err := d.decodeDimensionCodec(protocol, encoded.DimensionCodec); err != nil {
		return err
	}
	if err := d.decodeDimension(protocol, encoded.Dimension); err != nil {
		return err
	}
	return nil
}

func (d *Data) decodeRegistries(protocol int32, encodedRegistries []encodedRegistry) error {
	for _, encodedRegistry := range encodedRegistries {
		registry := Registry{ID: encodedRegistry.ID}
		for _, encodedEntry := range encodedRegistry.Entries {
			value, err := base64.StdEncoding.DecodeString(encodedEntry.Value)
			if err != nil {
				return fmt.Errorf("decode registry %d %s/%s: %w", protocol, encodedRegistry.ID, encodedEntry.Key, err)
			}
			registry.Entries = append(registry.Entries, Entry{
				Key:   encodedEntry.Key,
				Value: value,
			})
		}
		d.registries[protocol] = append(d.registries[protocol], registry)
	}
	return nil
}

func (d *Data) decodeTags(protocol int32, encodedTagRegistries []encodedTagRegistry) {
	for _, encodedTagRegistry := range encodedTagRegistries {
		tagRegistry := TagRegistry{ID: encodedTagRegistry.ID}
		for _, encodedTag := range encodedTagRegistry.Tags {
			values := append([]int32(nil), encodedTag.Values...)
			tagRegistry.Tags = append(tagRegistry.Tags, Tag{
				Key:    encodedTag.Key,
				Values: values,
			})
		}
		d.tags[protocol] = append(d.tags[protocol], tagRegistry)
	}
}

func (d *Data) decodeDimensionCodec(protocol int32, rawCodec string) error {
	if rawCodec == "" {
		return nil
	}
	codec, err := base64.StdEncoding.DecodeString(rawCodec)
	if err != nil {
		return fmt.Errorf("decode dimension codec %d: %w", protocol, err)
	}
	d.codecs[protocol] = codec
	return nil
}

func (d *Data) decodeDimension(protocol int32, rawDimension string) error {
	if rawDimension == "" {
		return nil
	}
	dimension, err := base64.StdEncoding.DecodeString(rawDimension)
	if err != nil {
		return fmt.Errorf("decode dimension %d: %w", protocol, err)
	}
	d.dimensions[protocol] = dimension
	return nil
}

func isZip(data []byte) bool {
	return len(data) >= 4 && bytes.Equal(data[:4], []byte{'P', 'K', 0x03, 0x04})
}

func protocolName(name string) string {
	base := filepath.Base(name)
	return strings.TrimSuffix(base, filepath.Ext(base))
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
