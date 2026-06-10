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

func TestProtocol774IncludesRequiredDamageTypes(t *testing.T) {
	registries, ok := Registries(774)
	if !ok {
		t.Fatalf("protocol 774 has no generated registries")
	}
	damageTypes := map[string]bool{}
	for _, registry := range registries {
		if registry.ID != "minecraft:damage_type" {
			continue
		}
		for _, entry := range registry.Entries {
			damageTypes[entry.Key] = true
		}
	}
	for _, key := range []string{
		"minecraft:generic",
		"minecraft:in_fire",
		"minecraft:on_fire",
		"minecraft:out_of_world",
	} {
		if !damageTypes[key] {
			t.Fatalf("protocol 774 missing damage type %s", key)
		}
	}
}

func TestProtocol774IncludesWorldRuntimeRegistryEntries(t *testing.T) {
	registries, ok := Registries(774)
	if !ok {
		t.Fatalf("protocol 774 has no generated registries")
	}
	entries := map[string]map[string]bool{}
	for _, registry := range registries {
		entries[registry.ID] = map[string]bool{}
		for _, entry := range registry.Entries {
			entries[registry.ID][entry.Key] = true
		}
	}
	cases := map[string][]string{
		"minecraft:worldgen/biome": {"minecraft:plains", "minecraft:forest", "minecraft:dark_forest"},
		"minecraft:banner_pattern": {"minecraft:base", "minecraft:border", "minecraft:stripe_bottom"},
		"minecraft:dialog":         {"minecraft:server_links", "minecraft:quick_actions"},
	}
	for registryID, keys := range cases {
		for _, key := range keys {
			if !entries[registryID][key] {
				t.Fatalf("protocol 774 missing %s/%s", registryID, key)
			}
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

func TestProtocol774IncludesRequiredGeneratedTags(t *testing.T) {
	tags, ok := Tags(774)
	if !ok {
		t.Fatalf("protocol 774 has no generated tags")
	}
	for _, registry := range tags {
		if registry.ID != "minecraft:item" {
			continue
		}
		for _, tag := range registry.Tags {
			if tag.Key == "minecraft:enchantable/head_armor" && len(tag.Values) > 0 {
				return
			}
		}
	}
	t.Fatalf("protocol 774 missing non-empty minecraft:item/minecraft:enchantable/head_armor tag")
}

func TestProtocol774BindsEnchantmentReferencedTags(t *testing.T) {
	tags, ok := Tags(774)
	if !ok {
		t.Fatalf("protocol 774 has no generated tags")
	}
	present := map[string]bool{}
	for _, registry := range tags {
		for _, tag := range registry.Tags {
			present[registry.ID+"/"+tag.Key] = true
		}
	}
	for _, key := range []string{
		"minecraft:block/minecraft:soul_speed_blocks",
		"minecraft:block/minecraft:blocks_wind_charge_explosions",
		"minecraft:entity_type/minecraft:sensitive_to_smite",
		"minecraft:entity_type/minecraft:arrows",
	} {
		if !present[key] {
			t.Fatalf("protocol 774 missing generated tag %s", key)
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
