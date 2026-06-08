package main

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

const (
	nbtEnd       byte = 0
	nbtByte      byte = 1
	nbtShort     byte = 2
	nbtInt       byte = 3
	nbtLong      byte = 4
	nbtFloat     byte = 5
	nbtDouble    byte = 6
	nbtByteArray byte = 7
	nbtString    byte = 8
	nbtList      byte = 9
	nbtCompound  byte = 10
	nbtIntArray  byte = 11
	nbtLongArray byte = 12
)

var wantedEntries = map[string][]string{
	"minecraft:worldgen/biome": {"minecraft:plains"},
	"minecraft:chat_type":      {"minecraft:chat"},
	"minecraft:damage_type":    {"minecraft:generic"},
}

var registryOrder = []string{
	"minecraft:worldgen/biome",
	"minecraft:chat_type",
	"minecraft:damage_type",
}

type versionJSON struct {
	Version          int32  `json:"version"`
	MinecraftVersion string `json:"minecraftVersion"`
	MajorVersion     string `json:"majorVersion"`
	ReleaseType      string `json:"releaseType"`
}

type protocolVersionJSON struct {
	MinecraftVersion string `json:"minecraftVersion"`
	Version          int32  `json:"version"`
	ReleaseType      string `json:"releaseType"`
}

type loginPacketJSON struct {
	DimensionCodec json.RawMessage `json:"dimensionCodec"`
	Dimension      nbtValue        `json:"dimension"`
}

type registryJSON struct {
	ID      string      `json:"id"`
	Entries []entryJSON `json:"entries"`
}

type entryJSON struct {
	Key   string   `json:"key"`
	Value nbtValue `json:"value"`
}

type nbtValue struct {
	Type  string          `json:"type"`
	Value json.RawMessage `json:"value"`
}

type generatedRegistry struct {
	ID      string
	Entries []generatedEntry
}

type generatedEntry struct {
	Key   string
	Value []byte
}

type generatedData struct {
	Registries map[int32][]generatedRegistry
	Codecs     map[int32][]byte
	Dimensions map[int32][]byte
}

type encodedData struct {
	Registries      map[string][]encodedRegistry `json:"registries"`
	DimensionCodecs map[string]string            `json:"dimension_codecs"`
	Dimensions      map[string]string            `json:"dimensions"`
}

type encodedRegistry struct {
	ID      string         `json:"id"`
	Entries []encodedEntry `json:"entries"`
}

type encodedEntry struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

func main() {
	var pcDataDir string
	var outPath string
	flag.StringVar(&pcDataDir, "pc-data", "", "path to minecraft-data/data/pc")
	flag.StringVar(&outPath, "out", "", "output Go file")
	flag.Parse()

	if pcDataDir == "" || outPath == "" {
		fatalf("-pc-data and -out are required")
	}
	data, err := readProtocolRegistries(pcDataDir)
	if err != nil {
		fatalf("%v", err)
	}
	source, err := render(data)
	if err != nil {
		fatalf("%v", err)
	}
	if err := os.WriteFile(outPath, source, 0o644); err != nil {
		fatalf("write %s: %v", outPath, err)
	}
}

func readProtocolRegistries(pcDataDir string) (generatedData, error) {
	orderPath := filepath.Join(pcDataDir, "common", "versions.json")
	data, err := os.ReadFile(orderPath)
	if err != nil {
		return generatedData{}, fmt.Errorf("read version order: %w", err)
	}
	var orderedDirs []string
	if err := json.Unmarshal(data, &orderedDirs); err != nil {
		return generatedData{}, fmt.Errorf("parse version order: %w", err)
	}
	releaseTypes, err := readReleaseTypes(pcDataDir)
	if err != nil {
		return generatedData{}, err
	}

	out := generatedData{
		Registries: map[int32][]generatedRegistry{},
		Codecs:     map[int32][]byte{},
		Dimensions: map[int32][]byte{},
	}
	var latest []generatedRegistry
	var latestCodec []byte
	var latestDimension []byte
	for _, dir := range orderedDirs {
		version, err := readVersion(filepath.Join(pcDataDir, dir, "version.json"))
		if err != nil || version.Version <= 0 || version.MinecraftVersion == "" {
			continue
		}
		version.ReleaseType = releaseTypeFor(version, releaseTypes)
		if version.ReleaseType != "release" || !isModernJava(version.MajorVersion) || !isReleaseName(version.MinecraftVersion) {
			continue
		}
		loginPath := filepath.Join(pcDataDir, dir, "loginPacket.json")
		if registries, codec, dimension, err := readLoginRegistries(loginPath); err == nil {
			if registries != nil {
				latest = registries
			}
			if codec != nil {
				latestCodec = codec
			}
			if dimension != nil {
				latestDimension = dimension
			}
		} else if version.Version >= 757 && !os.IsNotExist(err) {
			return generatedData{}, err
		}
		if latest != nil {
			out.Registries[version.Version] = cloneRegistries(latest)
		}
		if latestCodec != nil && version.Version >= 757 && version.Version < 766 {
			out.Codecs[version.Version] = append([]byte(nil), latestCodec...)
		}
		if latestDimension != nil && version.Version >= 757 && version.Version < 759 {
			out.Dimensions[version.Version] = append([]byte(nil), latestDimension...)
		}
	}
	if len(out.Registries) == 0 && len(out.Codecs) == 0 {
		return generatedData{}, fmt.Errorf("no registry data found in %s", pcDataDir)
	}
	return out, nil
}

func readLoginRegistries(path string) ([]generatedRegistry, []byte, []byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, nil, err
	}
	var login loginPacketJSON
	if err := json.Unmarshal(data, &login); err != nil {
		return nil, nil, nil, fmt.Errorf("parse %s: %w", path, err)
	}
	if len(login.DimensionCodec) == 0 {
		return nil, nil, nil, fmt.Errorf("%s missing dimensionCodec", path)
	}

	var dimension []byte
	if login.Dimension.Type != "" {
		encoded, err := encodeAnonymousNBT(login.Dimension)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("%s encode dimension: %w", path, err)
		}
		dimension = encoded
	}

	var typed nbtValue
	if err := json.Unmarshal(login.DimensionCodec, &typed); err == nil && typed.Type != "" {
		codec, err := encodeAnonymousNBT(typed)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("%s encode dimensionCodec: %w", path, err)
		}
		return nil, codec, dimension, nil
	}

	var dimensionCodec map[string]registryJSON
	if err := json.Unmarshal(login.DimensionCodec, &dimensionCodec); err != nil {
		return nil, nil, nil, fmt.Errorf("parse %s dimensionCodec: %w", path, err)
	}
	var registries []generatedRegistry
	for _, registryID := range registryOrder {
		registry, ok := dimensionCodec[registryID]
		if !ok {
			return nil, nil, nil, fmt.Errorf("%s missing registry %s", path, registryID)
		}
		generated := generatedRegistry{ID: registry.ID}
		if generated.ID == "" {
			generated.ID = registryID
		}
		for _, key := range wantedEntries[registryID] {
			entry, ok := findEntry(registry.Entries, key)
			if !ok {
				return nil, nil, nil, fmt.Errorf("%s missing registry entry %s/%s", path, registryID, key)
			}
			value, err := encodeAnonymousNBT(entry.Value)
			if err != nil {
				return nil, nil, nil, fmt.Errorf("%s encode %s/%s: %w", path, registryID, key, err)
			}
			generated.Entries = append(generated.Entries, generatedEntry{
				Key:   key,
				Value: value,
			})
		}
		registries = append(registries, generated)
	}
	return registries, nil, dimension, nil
}

func findEntry(entries []entryJSON, key string) (entryJSON, bool) {
	for _, entry := range entries {
		if entry.Key == key {
			return entry, true
		}
	}
	return entryJSON{}, false
}

func encodeAnonymousNBT(value nbtValue) ([]byte, error) {
	tag, err := tagID(value.Type)
	if err != nil {
		return nil, err
	}
	if tag != nbtCompound {
		return nil, fmt.Errorf("root must be compound, got %s", value.Type)
	}
	var buf bytes.Buffer
	_ = buf.WriteByte(tag)
	if err := writePayload(&buf, tag, value); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func writeNamedValue(buf *bytes.Buffer, name string, value nbtValue) error {
	tag, err := tagID(value.Type)
	if err != nil {
		return err
	}
	_ = buf.WriteByte(tag)
	if tag == nbtEnd {
		return nil
	}
	writeRawString(buf, name)
	return writePayload(buf, tag, value)
}

func writePayload(buf *bytes.Buffer, tag byte, value nbtValue) error {
	switch tag {
	case nbtEnd:
		return nil
	case nbtByte:
		v, err := intValue(value.Value)
		if err != nil {
			return err
		}
		_ = buf.WriteByte(byte(int8(v)))
	case nbtShort:
		v, err := intValue(value.Value)
		if err != nil {
			return err
		}
		writeRawShort(buf, int16(v))
	case nbtInt:
		v, err := intValue(value.Value)
		if err != nil {
			return err
		}
		writeRawInt(buf, int32(v))
	case nbtLong:
		v, err := intValue(value.Value)
		if err != nil {
			return err
		}
		writeRawLong(buf, int64(v))
	case nbtFloat:
		v, err := floatValue(value.Value)
		if err != nil {
			return err
		}
		writeRawFloat(buf, float32(v))
	case nbtDouble:
		v, err := floatValue(value.Value)
		if err != nil {
			return err
		}
		writeRawDouble(buf, v)
	case nbtString:
		var s string
		if err := json.Unmarshal(value.Value, &s); err != nil {
			return err
		}
		writeRawString(buf, s)
	case nbtList:
		return writeListPayload(buf, value.Value)
	case nbtCompound:
		var fields map[string]nbtValue
		if err := json.Unmarshal(value.Value, &fields); err != nil {
			return err
		}
		names := make([]string, 0, len(fields))
		for name := range fields {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			if err := writeNamedValue(buf, name, fields[name]); err != nil {
				return err
			}
		}
		_ = buf.WriteByte(nbtEnd)
	case nbtByteArray:
		values, err := intArrayValue(value.Value)
		if err != nil {
			return err
		}
		writeRawInt(buf, int32(len(values)))
		for _, value := range values {
			_ = buf.WriteByte(byte(int8(value)))
		}
	case nbtIntArray:
		values, err := intArrayValue(value.Value)
		if err != nil {
			return err
		}
		writeRawInt(buf, int32(len(values)))
		for _, value := range values {
			writeRawInt(buf, int32(value))
		}
	case nbtLongArray:
		values, err := intArrayValue(value.Value)
		if err != nil {
			return err
		}
		writeRawInt(buf, int32(len(values)))
		for _, value := range values {
			writeRawLong(buf, int64(value))
		}
	default:
		return fmt.Errorf("unsupported tag id %d", tag)
	}
	return nil
}

func writeListPayload(buf *bytes.Buffer, raw json.RawMessage) error {
	var list struct {
		Type  string            `json:"type"`
		Value []json.RawMessage `json:"value"`
	}
	if err := json.Unmarshal(raw, &list); err != nil {
		return err
	}
	tag, err := tagID(list.Type)
	if err != nil {
		return err
	}
	_ = buf.WriteByte(tag)
	writeRawInt(buf, int32(len(list.Value)))
	for _, rawValue := range list.Value {
		if err := writePayload(buf, tag, nbtValue{Type: list.Type, Value: rawValue}); err != nil {
			return err
		}
	}
	return nil
}

func tagID(name string) (byte, error) {
	switch name {
	case "end":
		return nbtEnd, nil
	case "byte":
		return nbtByte, nil
	case "short":
		return nbtShort, nil
	case "int":
		return nbtInt, nil
	case "long":
		return nbtLong, nil
	case "float":
		return nbtFloat, nil
	case "double":
		return nbtDouble, nil
	case "byteArray":
		return nbtByteArray, nil
	case "string":
		return nbtString, nil
	case "list":
		return nbtList, nil
	case "compound":
		return nbtCompound, nil
	case "intArray":
		return nbtIntArray, nil
	case "longArray":
		return nbtLongArray, nil
	default:
		return 0, fmt.Errorf("unsupported nbt type %q", name)
	}
}

func intValue(raw json.RawMessage) (int64, error) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var value json.Number
	if err := decoder.Decode(&value); err == nil {
		return value.Int64()
	}

	decoder = json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var pair []json.Number
	if err := decoder.Decode(&pair); err != nil {
		return 0, err
	}
	if len(pair) != 2 {
		return 0, fmt.Errorf("integer array has %d elements, want 2", len(pair))
	}
	hi, err := pair[0].Int64()
	if err != nil {
		return 0, err
	}
	lo, err := pair[1].Int64()
	if err != nil {
		return 0, err
	}
	return int64(uint64(uint32(hi))<<32 | uint64(uint32(lo))), nil
}

func floatValue(raw json.RawMessage) (float64, error) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var value json.Number
	if err := decoder.Decode(&value); err != nil {
		return 0, err
	}
	return value.Float64()
}

func intArrayValue(raw json.RawMessage) ([]int64, error) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var rawValues []json.Number
	if err := decoder.Decode(&rawValues); err != nil {
		return nil, err
	}
	values := make([]int64, 0, len(rawValues))
	for _, rawValue := range rawValues {
		value, err := rawValue.Int64()
		if err != nil {
			return nil, err
		}
		values = append(values, value)
	}
	return values, nil
}

func writeRawShort(buf *bytes.Buffer, value int16) {
	var b [2]byte
	binary.BigEndian.PutUint16(b[:], uint16(value))
	_, _ = buf.Write(b[:])
}

func writeRawInt(buf *bytes.Buffer, value int32) {
	var b [4]byte
	binary.BigEndian.PutUint32(b[:], uint32(value))
	_, _ = buf.Write(b[:])
}

func writeRawLong(buf *bytes.Buffer, value int64) {
	var b [8]byte
	binary.BigEndian.PutUint64(b[:], uint64(value))
	_, _ = buf.Write(b[:])
}

func writeRawFloat(buf *bytes.Buffer, value float32) {
	var b [4]byte
	binary.BigEndian.PutUint32(b[:], math.Float32bits(value))
	_, _ = buf.Write(b[:])
}

func writeRawDouble(buf *bytes.Buffer, value float64) {
	var b [8]byte
	binary.BigEndian.PutUint64(b[:], math.Float64bits(value))
	_, _ = buf.Write(b[:])
}

func writeRawString(buf *bytes.Buffer, value string) {
	var b [2]byte
	binary.BigEndian.PutUint16(b[:], uint16(len(value)))
	_, _ = buf.Write(b[:])
	_, _ = io.WriteString(buf, value)
}

func readVersion(path string) (versionJSON, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return versionJSON{}, err
	}
	var version versionJSON
	if err := json.Unmarshal(data, &version); err != nil {
		return versionJSON{}, err
	}
	return version, nil
}

func readReleaseTypes(pcDataDir string) (map[string]string, error) {
	path := filepath.Join(pcDataDir, "common", "protocolVersions.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read protocol versions: %w", err)
	}
	var entries []protocolVersionJSON
	if err := json.Unmarshal(data, &entries); err != nil {
		return nil, fmt.Errorf("parse protocol versions: %w", err)
	}
	releaseTypes := make(map[string]string, len(entries)*2)
	for _, entry := range entries {
		releaseType := releaseType(entry.ReleaseType)
		releaseTypes[entry.MinecraftVersion] = releaseType
		releaseTypes[entry.MinecraftVersion+"/"+strconv.Itoa(int(entry.Version))] = releaseType
	}
	return releaseTypes, nil
}

func render(data generatedData) ([]byte, error) {
	out := encodedData{
		Registries:      map[string][]encodedRegistry{},
		DimensionCodecs: map[string]string{},
		Dimensions:      map[string]string{},
	}
	ids := make([]int, 0, len(data.Registries))
	for protocol := range data.Registries {
		ids = append(ids, int(protocol))
	}
	sort.Ints(ids)
	for _, id := range ids {
		var registries []encodedRegistry
		for _, registry := range data.Registries[int32(id)] {
			encoded := encodedRegistry{ID: registry.ID}
			for _, entry := range registry.Entries {
				encoded.Entries = append(encoded.Entries, encodedEntry{
					Key:   entry.Key,
					Value: base64.StdEncoding.EncodeToString(entry.Value),
				})
			}
			registries = append(registries, encoded)
		}
		out.Registries[strconv.Itoa(id)] = registries
	}
	codecIDs := make([]int, 0, len(data.Codecs))
	for protocol := range data.Codecs {
		codecIDs = append(codecIDs, int(protocol))
	}
	sort.Ints(codecIDs)
	for _, id := range codecIDs {
		out.DimensionCodecs[strconv.Itoa(id)] = base64.StdEncoding.EncodeToString(data.Codecs[int32(id)])
	}
	dimensionIDs := make([]int, 0, len(data.Dimensions))
	for protocol := range data.Dimensions {
		dimensionIDs = append(dimensionIDs, int(protocol))
	}
	sort.Ints(dimensionIDs)
	for _, id := range dimensionIDs {
		out.Dimensions[strconv.Itoa(id)] = base64.StdEncoding.EncodeToString(data.Dimensions[int32(id)])
	}
	source, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("encode registry data: %w", err)
	}
	return append(source, '\n'), nil
}

func byteList(values []byte) string {
	parts := make([]string, 0, len(values))
	for _, value := range values {
		parts = append(parts, strconv.Itoa(int(value)))
	}
	return strings.Join(parts, ", ")
}

func byteStringLiteral(values []byte) string {
	if len(values) == 0 {
		return "nil"
	}
	var buf strings.Builder
	buf.WriteString("[]byte(")
	for start := 0; start < len(values); start += 128 {
		if start > 0 {
			buf.WriteString(" + ")
		}
		end := start + 128
		if end > len(values) {
			end = len(values)
		}
		buf.WriteString(fmt.Sprintf("%q", string(values[start:end])))
	}
	buf.WriteString(")")
	return buf.String()
}

func cloneRegistries(in []generatedRegistry) []generatedRegistry {
	out := make([]generatedRegistry, 0, len(in))
	for _, registry := range in {
		entries := make([]generatedEntry, 0, len(registry.Entries))
		for _, entry := range registry.Entries {
			entries = append(entries, generatedEntry{
				Key:   entry.Key,
				Value: append([]byte(nil), entry.Value...),
			})
		}
		out = append(out, generatedRegistry{
			ID:      registry.ID,
			Entries: entries,
		})
	}
	return out
}

func releaseType(value string) string {
	if value == "" {
		return "release"
	}
	return strings.ToLower(value)
}

func releaseTypeFor(version versionJSON, releaseTypes map[string]string) string {
	if releaseType, ok := releaseTypes[version.MinecraftVersion+"/"+strconv.Itoa(int(version.Version))]; ok {
		return releaseType
	}
	if releaseType, ok := releaseTypes[version.MinecraftVersion]; ok {
		return releaseType
	}
	return releaseType(version.ReleaseType)
}

func isModernJava(majorVersion string) bool {
	return strings.HasPrefix(majorVersion, "1.") || strings.HasPrefix(majorVersion, "26.")
}

func isReleaseName(minecraftVersion string) bool {
	if strings.Contains(minecraftVersion, "-pre") || strings.Contains(minecraftVersion, "-rc") {
		return false
	}
	for _, part := range strings.Split(minecraftVersion, ".") {
		if strings.Contains(part, "w") {
			return false
		}
	}
	return true
}

func fatalf(format string, args ...any) {
	_, _ = fmt.Fprintf(os.Stderr, "mcregistry-gen: "+format+"\n", args...)
	os.Exit(1)
}
