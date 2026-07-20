package service

import (
	"encoding/json"
	"path/filepath"
	"sort"
	"strings"
)

// PackageManager identifies a declarative install recipe. Resolution only
// inspects a repository snapshot; it never invokes the selected command.
type PackageManager string

const (
	PackageManagerUnknown  PackageManager = "unknown"
	PackageManagerNPM      PackageManager = "npm"
	PackageManagerPNPM     PackageManager = "pnpm"
	PackageManagerYarn     PackageManager = "yarn"
	PackageManagerBun      PackageManager = "bun"
	PackageManagerGo       PackageManager = "go"
	PackageManagerCargo    PackageManager = "cargo"
	PackageManagerPip      PackageManager = "pip"
	PackageManagerUV       PackageManager = "uv"
	PackageManagerPoetry   PackageManager = "poetry"
	PackageManagerPipenv   PackageManager = "pipenv"
	PackageManagerMaven    PackageManager = "maven"
	PackageManagerGradle   PackageManager = "gradle"
	PackageManagerComposer PackageManager = "composer"
	PackageManagerBundler  PackageManager = "bundler"
	PackageManagerDotnet   PackageManager = "dotnet"
	PackageManagerMix      PackageManager = "mix"
)

type PackageManagerInput struct {
	Files    map[string][]byte
	Metadata map[string]string
}

type PackageManagerResolution struct {
	Manager    PackageManager
	Recipe     Recipe
	Confidence Confidence
	Evidence   []string
}

func PackageManagerRecipe(manager PackageManager) Recipe {
	recipes := map[PackageManager][]string{
		PackageManagerNPM: {"npm", "ci"}, PackageManagerPNPM: {"pnpm", "install", "--frozen-lockfile"},
		PackageManagerYarn: {"yarn", "install", "--immutable"}, PackageManagerBun: {"bun", "install", "--frozen-lockfile"},
		PackageManagerGo: {"go", "mod", "download"}, PackageManagerCargo: {"cargo", "fetch"},
		PackageManagerPip: {"pip", "install", "-r", "requirements.txt"}, PackageManagerUV: {"uv", "sync", "--frozen"},
		PackageManagerPoetry: {"poetry", "install", "--no-interaction"}, PackageManagerPipenv: {"pipenv", "sync"},
		PackageManagerMaven: {"mvn", "dependency:go-offline"}, PackageManagerGradle: {"gradle", "dependencies"},
		PackageManagerComposer: {"composer", "install", "--no-interaction", "--prefer-dist"}, PackageManagerBundler: {"bundle", "install"},
		PackageManagerDotnet: {"dotnet", "restore"}, PackageManagerMix: {"mix", "deps.get"},
	}
	if install, ok := recipes[manager]; ok {
		return Recipe{Install: append([]string(nil), install...)}
	}
	return Recipe{Manual: true}
}

// ResolvePackageManager applies safe precedence: native manifest, lockfile,
// explicit metadata, then a low-confidence language fallback. Multiple
// managers in a single tier are deliberately manual rather than arbitrary.
func ResolvePackageManager(input PackageManagerInput) PackageManagerResolution {
	files := normalizeStackFiles(input.Files)
	if result, ok := resolveManagerTier("language-native manifest", managerCandidates(files, true)); ok {
		return result
	}
	if result, ok := resolveManagerTier("lockfile", lockCandidates(files)); ok {
		return result
	}
	if result, ok := resolveManagerTier("explicit metadata", metadataCandidates(files, input.Metadata)); ok {
		return result
	}
	if result, ok := resolveManagerTier("conservative fallback", fallbackCandidates(files)); ok {
		return result
	}
	return unknownManager("no package-manager manifest, lockfile, explicit metadata, or safe fallback was found")
}

type managerCandidate struct {
	manager  PackageManager
	evidence string
}

func managerCandidates(files map[string][]byte, native bool) []managerCandidate {
	var result []managerCandidate
	for path, content := range files {
		base := filepath.Base(path)
		var manager PackageManager
		switch {
		case base == "go.mod":
			manager = PackageManagerGo
		case base == "Cargo.toml":
			manager = PackageManagerCargo
		case base == "Pipfile":
			manager = PackageManagerPipenv
		case base == "pom.xml":
			manager = PackageManagerMaven
		case base == "composer.json":
			manager = PackageManagerComposer
		case base == "Gemfile":
			manager = PackageManagerBundler
		case base == "mix.exs":
			manager = PackageManagerMix
		case strings.HasSuffix(base, ".csproj") || strings.HasSuffix(base, ".fsproj"):
			manager = PackageManagerDotnet
		case base == "build.gradle" || base == "build.gradle.kts" || base == "settings.gradle" || base == "settings.gradle.kts":
			manager = PackageManagerGradle
		case base == "bunfig.toml":
			manager = PackageManagerBun
		case base == "pyproject.toml" && strings.Contains(string(content), "[tool.poetry]"):
			manager = PackageManagerPoetry
		case base == "pyproject.toml" && strings.Contains(string(content), "[tool.uv]"):
			manager = PackageManagerUV
		}
		if manager != "" {
			result = append(result, managerCandidate{manager, path})
		}
	}
	return uniqueManagerCandidates(result)
}

func lockCandidates(files map[string][]byte) []managerCandidate {
	locks := map[string]PackageManager{"package-lock.json": PackageManagerNPM, "npm-shrinkwrap.json": PackageManagerNPM, "pnpm-lock.yaml": PackageManagerPNPM, "yarn.lock": PackageManagerYarn, "bun.lock": PackageManagerBun, "bun.lockb": PackageManagerBun, "uv.lock": PackageManagerUV, "poetry.lock": PackageManagerPoetry, "Pipfile.lock": PackageManagerPipenv, "Cargo.lock": PackageManagerCargo, "composer.lock": PackageManagerComposer, "Gemfile.lock": PackageManagerBundler, "mix.lock": PackageManagerMix}
	var result []managerCandidate
	for path := range files {
		if manager := locks[filepath.Base(path)]; manager != "" {
			result = append(result, managerCandidate{manager, path})
		}
	}
	return uniqueManagerCandidates(result)
}

func metadataCandidates(files map[string][]byte, metadata map[string]string) []managerCandidate {
	var result []managerCandidate
	for key, value := range metadata {
		if strings.EqualFold(key, "packageManager") || strings.EqualFold(key, "package_manager") {
			if manager := managerFromValue(value); manager != PackageManagerUnknown {
				result = append(result, managerCandidate{manager, "metadata:" + key})
			}
		}
	}
	for path, content := range files {
		if filepath.Base(path) != "package.json" {
			continue
		}
		var pkg struct {
			PackageManager string `json:"packageManager"`
		}
		if json.Unmarshal(content, &pkg) == nil {
			if manager := managerFromValue(pkg.PackageManager); manager != PackageManagerUnknown {
				result = append(result, managerCandidate{manager, path + ":packageManager"})
			}
		}
	}
	return uniqueManagerCandidates(result)
}

func fallbackCandidates(files map[string][]byte) []managerCandidate {
	var result []managerCandidate
	for path := range files {
		switch base := filepath.Base(path); {
		case base == "package.json":
			result = append(result, managerCandidate{PackageManagerNPM, path})
		case base == "requirements.txt" || strings.HasPrefix(base, "requirements-") && strings.HasSuffix(base, ".txt") || base == "pyproject.toml":
			result = append(result, managerCandidate{PackageManagerPip, path})
		}
	}
	return uniqueManagerCandidates(result)
}

func resolveManagerTier(tier string, candidates []managerCandidate) (PackageManagerResolution, bool) {
	if len(candidates) == 0 {
		return PackageManagerResolution{}, false
	}
	if len(candidates) > 1 {
		evidence := []string{"conflicting " + tier + " candidates; manual configuration is required"}
		for _, candidate := range candidates {
			evidence = append(evidence, string(candidate.manager)+" from "+candidate.evidence)
		}
		return PackageManagerResolution{Manager: PackageManagerUnknown, Recipe: PackageManagerRecipe(PackageManagerUnknown), Confidence: ConfidenceUnknown, Evidence: evidence}, true
	}
	confidence := ConfidenceHigh
	if tier == "conservative fallback" {
		confidence = ConfidenceLow
	}
	candidate := candidates[0]
	return PackageManagerResolution{Manager: candidate.manager, Recipe: PackageManagerRecipe(candidate.manager), Confidence: confidence, Evidence: []string{string(candidate.manager) + " selected from " + tier + ": " + candidate.evidence}}, true
}

func unknownManager(reason string) PackageManagerResolution {
	return PackageManagerResolution{Manager: PackageManagerUnknown, Recipe: PackageManagerRecipe(PackageManagerUnknown), Confidence: ConfidenceUnknown, Evidence: []string{reason}}
}
func managerFromValue(value string) PackageManager {
	manager := PackageManager(strings.ToLower(strings.TrimSpace(strings.SplitN(value, "@", 2)[0])))
	if PackageManagerRecipe(manager).Manual {
		return PackageManagerUnknown
	}
	return manager
}
func uniqueManagerCandidates(candidates []managerCandidate) []managerCandidate {
	byManager := map[PackageManager][]string{}
	for _, candidate := range candidates {
		byManager[candidate.manager] = append(byManager[candidate.manager], candidate.evidence)
	}
	managers := make([]string, 0, len(byManager))
	for manager := range byManager {
		managers = append(managers, string(manager))
	}
	sort.Strings(managers)
	result := make([]managerCandidate, 0, len(managers))
	for _, name := range managers {
		evidence := byManager[PackageManager(name)]
		sort.Strings(evidence)
		result = append(result, managerCandidate{PackageManager(name), strings.Join(evidence, ", ")})
	}
	return result
}
