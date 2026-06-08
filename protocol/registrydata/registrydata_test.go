package registrydata

import "testing"

func TestRegistriesIncludeModernBaselineEntries(t *testing.T) {
	for _, protocol := range []int32{766, 774} {
		registries, ok := Registries(protocol)
		if !ok {
			t.Fatalf("protocol %d has no generated registries", protocol)
		}
		entries := map[string]bool{}
		for _, registry := range registries {
			for _, entry := range registry.Entries {
				if len(entry.Value) == 0 || entry.Value[0] != 10 {
					t.Fatalf("protocol %d entry %s/%s is not anonymous compound NBT", protocol, registry.ID, entry.Key)
				}
				entries[registry.ID+"/"+entry.Key] = true
			}
		}
		for _, key := range []string{
			"minecraft:worldgen/biome/minecraft:plains",
			"minecraft:chat_type/minecraft:chat",
			"minecraft:damage_type/minecraft:generic",
		} {
			if !entries[key] {
				t.Fatalf("protocol %d missing generated entry %s", protocol, key)
			}
		}
	}
}

func TestDimensionCodecIncludesConfigurationBaseline(t *testing.T) {
	for _, protocol := range []int32{757, 758, 759, 760, 761, 762, 763, 764, 765} {
		codec, ok := DimensionCodec(protocol)
		if !ok {
			t.Fatalf("protocol %d has no generated dimension codec", protocol)
		}
		if len(codec) == 0 || codec[0] != 10 {
			t.Fatalf("protocol %d dimension codec is not anonymous compound NBT", protocol)
		}
	}
}

func TestDimensionIncludesNBTBaseline(t *testing.T) {
	data, err := Default()
	if err != nil {
		t.Fatalf("load default registry data: %v", err)
	}
	for _, protocol := range []int32{757, 758} {
		dimension, ok := data.Dimension(protocol)
		if !ok {
			t.Fatalf("protocol %d has no generated dimension", protocol)
		}
		if len(dimension) == 0 || dimension[0] != 10 {
			t.Fatalf("protocol %d dimension is not anonymous compound NBT", protocol)
		}
	}
}
