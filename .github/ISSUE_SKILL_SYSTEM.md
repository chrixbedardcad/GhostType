# Rename "Prompts" to "Skills" and introduce Skill Sets

## Summary

Rename the entire "Prompt" concept to "Skill" throughout the codebase (Go backend, TypeScript frontend, config JSON, tests). A **Skill** better describes what the feature actually is: not just a text template, but an **action with behavior** — it defines what input to capture, what to send to the LLM, and how to act on the result (replace text, open popup, copy to clipboard, etc.).

Additionally, introduce **Skill Sets** — named, toggleable collections of skills that let users quickly switch between different workflows (e.g. "Writing", "Developer", "Fun").

---

## Motivation

- "Prompt" implies raw text for an LLM. "Skill" implies a complete action with behavior.
- Each entry already has `display_mode`, `vision`, per-skill LLM override, and timeout — these are behavioral properties, not just prompt text.
- Users think in terms of "what does this tool *do*" not "what text am I sending."
- Aligns terminology with the broader AI tooling ecosystem (e.g. Claude Code skills).
- Skill Sets enable quick workflow switching without manually enabling/disabling individual items.

---

## Phase 1: Rename Prompt → Skill (no behavior change)

### Go Backend — Data Model (`config/config.go`)

| Line | Current | New |
|------|---------|-----|
| 13-22 | `type PromptEntry struct` | `type Skill struct` |
| 81 | `ActivePrompt int \`json:"active_prompt"\`` | `ActiveSkill int \`json:"active_skill"\`` |
| 82 | `Prompts []PromptEntry \`json:"prompts"\`` | `Skills []Skill \`json:"skills"\`` |
| 122 | `func DefaultPrompts() []PromptEntry` | `func DefaultSkills() []Skill` |
| 146 | `ActivePrompt: 0` | `ActiveSkill: 0` |
| 315 | `ActivePrompt` in legacy struct | Update migration code |
| 398 | `ActivePrompt: old.ActivePrompt` | `ActiveSkill: old.ActivePrompt` (migration) |
| 434 | `activePrompt` variable in migration | `activeSkill` |
| 448 | `ActivePrompt: activePrompt` | `ActiveSkill: activeSkill` |
| 580-582 | Clamp `ActivePrompt` | Clamp `ActiveSkill` |

**Keep backward compatibility**: When loading config, if `"prompts"` key exists but `"skills"` does not, auto-migrate by reading `"prompts"` into `Skills` and `"active_prompt"` into `ActiveSkill`.

### Go Backend — Service Layer (`gui/service_config.go`)

| Line | Current | New |
|------|---------|-----|
| 619-637 | `func SavePrompt(...)` | `func SaveSkill(...)` |
| 638-653 | `func DeletePrompt(...)` | `func DeleteSkill(...)` |
| 655-673 | `func MovePrompt(...)` | `func MoveSkill(...)` |
| 675-680 | `func ResetPrompts()` | `func ResetSkills()` |

All internal references to `s.cfgCopy.Prompts` → `s.cfgCopy.Skills` and `s.cfgCopy.ActivePrompt` → `s.cfgCopy.ActiveSkill`.

### Go Backend — Service (`gui/service.go`)

| Line | Current | New |
|------|---------|-----|
| 548 | `func CyclePromptFromIndicator()` | `func CycleSkillFromIndicator()` |
| 553-554 | `ActivePrompt`, `Prompts` refs | `ActiveSkill`, `Skills` |
| 599 | `Active: i == s.cfgCopy.ActivePrompt` | `Active: i == s.cfgCopy.ActiveSkill` |
| 607-613 | `func SetActivePromptFromIndicator(idx)` | `func SetActiveSkillFromIndicator(idx)` |
| 620-629 | `func GetActivePromptInfo()` | `func GetActiveSkillInfo()` |

### Go Backend — Router (`mode/router.go`)

| Line | Current | New |
|------|---------|-----|
| 188 | `func TimeoutForPrompt(promptIdx int)` | `func TimeoutForSkill(skillIdx int)` |
| 211 | `func llmLabelForPrompt(promptIdx int)` | `func llmLabelForSkill(skillIdx int)` |
| 223 | `func CyclePrompt()` | `func CycleSkill()` |
| 228-229 | `ActivePrompt` refs | `ActiveSkill` |
| 236 | `func SetPrompt(idx int)` | `func SetSkill(idx int)` |
| 247 | `func CurrentPromptIdx()` | `func CurrentSkillIdx()` |
| 254 | `func CurrentPromptName()` | `func CurrentSkillName()` |

### Go Backend — Other files

- **`app.go:45-49,191,269,272,452`** — `setActivePrompt` → `setActiveSkill`, all `ActivePrompt`/`Prompts` refs
- **`main.go:350-358`** — `ActivePrompt`/`Prompts` refs → `ActiveSkill`/`Skills`
- **`tray/tray.go:48,287-288,341`** — `GetActivePrompt` callback → `GetActiveSkill`
- **`gui/benchmark.go:76-79,289-292`** — `ActivePrompt`/`Prompts` refs
- **`gui/indicator.go`** — any prompt references
- **`capture.go`** — prompt references
- **`process.go`** — prompt references
- **`stats/stats.go`** — if it tracks prompt usage, rename

### Go Tests

- **`mode/router_test.go:44,159,188,227,242,301,338`** — all `Prompt`/`ActivePrompt` refs
- **`ghosttype_test.go:47,98,212,254`** — `ActivePrompt` refs
- **`config/config_test.go:31-32,768-790`** — `ActivePrompt` test assertions

### TypeScript Frontend

**`gui/frontend/src/windows/Settings/PromptsTab.tsx`** → rename to **`SkillsTab.tsx`**:

| Line | Current | New |
|------|---------|-----|
| 5-13 | `interface Prompt` | `interface Skill` |
| 16-18 | `PromptsTab` component, "Prompts tab" comment | `SkillsTab`, "Skills tab" |
| 20 | `useState<Prompt[]>` | `useState<Skill[]>` |
| All | `goCall("savePrompt"...)` | `goCall("saveSkill"...)` |
| All | `goCall("deletePrompt"...)` | `goCall("deleteSkill"...)` |
| All | `goCall("movePrompt"...)` | `goCall("moveSkill"...)` |
| All | `goCall("resetPrompts")` | `goCall("resetSkills")` |
| All | UI labels: "Add Prompt", "Delete Prompt", etc. | "Add Skill", "Delete Skill", etc. |

**`gui/frontend/src/windows/Settings/Settings.tsx`**:
- Tab label "Prompts" → "Skills"
- Import `PromptsTab` → `SkillsTab`

**Other frontend files** referencing "prompt":
- `gui/frontend/src/windows/Indicator.tsx`
- `gui/frontend/src/windows/Result.tsx`
- `gui/frontend/src/windows/Settings/HotkeysTab.tsx` — cycle_prompt hotkey label
- `gui/frontend/src/windows/Settings/StatsTab.tsx`
- `gui/frontend/src/windows/Settings/HelpTab.tsx`
- `gui/frontend/src/windows/Wizard/ReadyStep.tsx`

### Config JSON

```jsonc
// Before:
{ "active_prompt": 0, "prompts": [...] }

// After:
{ "active_skill": 0, "skills": [...] }
```

Migration: on load, if `"prompts"` key exists, copy to `"skills"` and remove `"prompts"`. Same for `"active_prompt"` → `"active_skill"`.

---

## Phase 2: Add `id` and `enabled` fields to Skill

### Data Model

```go
type Skill struct {
    ID          string `json:"id"`                      // unique slug, e.g. "correct", "my-custom-tone"
    Name        string `json:"name"`
    Prompt      string `json:"prompt"`
    Icon        string `json:"icon,omitempty"`
    LLM         string `json:"llm,omitempty"`
    TimeoutMs   int    `json:"timeout_ms,omitempty"`
    DisplayMode string `json:"display_mode,omitempty"`
    Vision      bool   `json:"vision,omitempty"`
    BuiltIn     bool   `json:"built_in,omitempty"`      // NEW: true for default skills
    Enabled     bool   `json:"enabled"`                  // NEW: appears in hotkey cycle
}
```

### Changes

- **`config/config.go`**: Add `ID`, `BuiltIn`, `Enabled` fields to `Skill` struct
- **`config/config.go` `DefaultSkills()`**: Set `ID` and `BuiltIn: true`, `Enabled: true` on all defaults
- **`config/config.go`**: Add `GenerateSkillID(name string) string` helper (slugify: lowercase, replace spaces with hyphens)
- **Migration**: Existing skills without `id` get one generated from their name. All migrated skills get `enabled: true`
- **`mode/router.go` `CycleSkill()`**: Only cycle through `enabled == true` skills
- **`gui/service.go` `CycleSkillFromIndicator()`**: Same — skip disabled skills
- **`tray/tray.go`**: Show enabled/disabled state in menu
- **Frontend `SkillsTab.tsx`**: Add enable/disable toggle per skill

---

## Phase 3: Introduce Skill Sets

### Data Model

```go
type SkillSet struct {
    ID          string   `json:"id"`              // unique slug
    Name        string   `json:"name"`            // display name
    Icon        string   `json:"icon,omitempty"`
    Description string   `json:"description,omitempty"`
    SkillIDs    []string `json:"skill_ids"`       // references Skill.ID
    BuiltIn     bool     `json:"built_in,omitempty"`
}
```

Add to Config:
```go
type Config struct {
    ActiveSkill int        `json:"active_skill"`
    Skills      []Skill    `json:"skills"`
    SkillSets   []SkillSet `json:"skill_sets,omitempty"`  // NEW
    // ...
}
```

### Default Skill Sets

| ID | Name | Icon | Skills |
|----|------|------|--------|
| `writing` | Writing | ✍️ | correct, polish, elaborate, shorten |
| `fun` | Fun | 😄 | funny, translate |
| `tools` | Tools | 🔧 | ask, define |
| `vision` | Vision | 📸 | describe-screenshot, screenshot-ocr |

### Backend Operations (`gui/service_config.go`)

New methods:
- `SaveSkillSet(id, name, icon, description string, skillIDs []string) string`
- `DeleteSkillSet(id string) string`
- `ActivateSkillSet(id string) string` — enables all skills in the set
- `DeactivateSkillSet(id string) string` — disables skills unique to this set

### Frontend — Redesigned Skills Tab

```
┌─────────────────────────────────────────────────┐
│  Skills                                          │
├─────────────────────────────────────────────────┤
│                                                  │
│  SKILL SETS                        [+ New Set]   │
│  ┌───────────┐ ┌───────────┐ ┌───────────┐     │
│  │ ✍️ Writing │ │ 🔧 Tools  │ │ ⚡ My Set  │     │
│  │ 4 skills  │ │ 2 skills  │ │ 3 skills  │     │
│  │ [Active]  │ │ [Enable]  │ │ [Enable]  │     │
│  └───────────┘ └───────────┘ └───────────┘     │
│                                                  │
│  ALL SKILLS                       [+ New Skill]  │
│  ┌──────────────────────────────────────────┐   │
│  │ ✏️ Correct          ● enabled     [edit] │   │
│  │ 💎 Polish           ● enabled     [edit] │   │
│  │ 🎨 My Tone          ● enabled     [edit] │   │
│  │ 😄 Funny            ○ disabled    [edit] │   │
│  └──────────────────────────────────────────┘   │
│                                                  │
└─────────────────────────────────────────────────┘
```

New components:
- `SkillSetCard.tsx` — card showing set name, icon, skill count, activate/deactivate button
- `SkillSetEditor.tsx` — create/edit a skill set (pick skills from list)

### User Flows

| Action | Behavior |
|--------|----------|
| Enable a Skill Set | All skills in the set get `enabled: true` |
| Disable a Skill Set | Skills unique to this set get `enabled: false` |
| Create a Skill | Adds to "All Skills", optionally assign to a set |
| Create a Skill Set | Pick existing skills to group |
| Cycle hotkey (Ctrl+J) | Only cycles `enabled: true` skills |
| Delete built-in skill | Not allowed — only disable |
| Reset | Restores all built-in skills and sets to defaults |

---

## Phase 4 (Future): Import/Export & Sharing

- Export a skill or skill set as `.ghostspell-skills.json`
- Import from file
- Optional: community gallery / curated skill packs

---

## Files to Modify (complete list)

### Go (non-vendor)
1. `config/config.go` — `PromptEntry` → `Skill`, `ActivePrompt` → `ActiveSkill`, `Prompts` → `Skills`, migration logic
2. `config/config_test.go` — update all test references
3. `config/defaults_darwin.go` — if it references prompts
4. `config/defaults_other.go` — if it references prompts
5. `gui/service_config.go` — `SavePrompt` → `SaveSkill`, `DeletePrompt` → `DeleteSkill`, etc.
6. `gui/service.go` — `CyclePromptFromIndicator` → `CycleSkillFromIndicator`, etc.
7. `gui/benchmark.go` — `ActivePrompt`/`Prompts` refs
8. `gui/indicator.go` — prompt references
9. `gui/result.go` — prompt references
10. `gui/catalog.go` — if it references prompts
11. `mode/router.go` — all prompt functions
12. `mode/router_test.go` — all prompt test functions
13. `tray/tray.go` — `GetActivePrompt` callback
14. `app.go` — `setActivePrompt`, all prompt refs
15. `main.go` — prompt refs in CLI output
16. `capture.go` — prompt refs
17. `process.go` — prompt refs
18. `ghosttype_test.go` — prompt test refs
19. `stats/stats.go` — if applicable

### TypeScript/Frontend
1. `gui/frontend/src/windows/Settings/PromptsTab.tsx` → rename to `SkillsTab.tsx`
2. `gui/frontend/src/windows/Settings/Settings.tsx` — tab label + import
3. `gui/frontend/src/windows/Indicator.tsx` — prompt refs
4. `gui/frontend/src/windows/Result.tsx` — prompt refs
5. `gui/frontend/src/windows/Settings/HotkeysTab.tsx` — cycle_prompt label
6. `gui/frontend/src/windows/Settings/StatsTab.tsx` — prompt stats
7. `gui/frontend/src/windows/Settings/HelpTab.tsx` — prompt docs
8. `gui/frontend/src/windows/Wizard/ReadyStep.tsx` — prompt refs

### Config JSON
- `config.json` schema: `prompts` → `skills`, `active_prompt` → `active_skill`
- Backward-compatible migration on load

---

## Acceptance Criteria

- [ ] All references to "prompt" (in the context of user-facing skill entries) are renamed to "skill" in Go, TypeScript, and JSON config
- [ ] Config migration: old `prompts`/`active_prompt` keys are auto-migrated to `skills`/`active_skill` on load
- [ ] All existing tests pass with updated naming
- [ ] `Skill` struct has `id`, `built_in`, and `enabled` fields
- [ ] Hotkey cycling respects `enabled` flag
- [ ] `SkillSet` model exists with CRUD operations
- [ ] UI shows skill sets at top of Skills tab, individual skills below
- [ ] Users can create custom skills and skill sets
- [ ] Built-in skills cannot be deleted, only disabled
- [ ] Default skill sets ship with the app (Writing, Fun, Tools, Vision)
