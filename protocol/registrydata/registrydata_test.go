package registrydata

import "testing"

func TestRegistriesIncludeModernBaselineEntries(t *testing.T) {
	for _, protocol := range []int32{766, 767, 768, 769, 770, 771, 772, 773, 774} {
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

func TestProtocol774IncludesVanillaRequiredVariantRegistries(t *testing.T) {
	registries, ok := Registries(774)
	if !ok {
		t.Fatalf("protocol 774 has no generated registries")
	}
	entries := map[string]int{}
	for _, registry := range registries {
		entries[registry.ID] = len(registry.Entries)
	}
	for _, registryID := range []string{
		"minecraft:cat_variant",
		"minecraft:chicken_variant",
		"minecraft:cow_variant",
		"minecraft:frog_variant",
		"minecraft:painting_variant",
		"minecraft:pig_variant",
		"minecraft:wolf_sound_variant",
		"minecraft:wolf_variant",
		"minecraft:zombie_nautilus_variant",
	} {
		if entries[registryID] == 0 {
			t.Fatalf("protocol 774 missing non-empty registry %s", registryID)
		}
	}
}

func TestGeneratedRegistrySetsAreNonEmpty(t *testing.T) {
	data, err := Default()
	if err != nil {
		t.Fatalf("load default registry data: %v", err)
	}
	for protocol, registries := range data.registries {
		for _, registry := range registries {
			if len(registry.Entries) == 0 {
				t.Fatalf("protocol %d registry %s has no entries", protocol, registry.ID)
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
