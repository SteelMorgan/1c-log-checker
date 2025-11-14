# Setup Claude Code Skills
# This script creates symbolic links to external skill directories

$userHome = [Environment]::GetFolderPath('UserProfile')
$claudeSkillsPath = Join-Path $userHome '.claude\skills'

Write-Host "Creating .claude\skills directory..."
New-Item -ItemType Directory -Force -Path $claudeSkillsPath | Out-Null

$skillsBasePath = "D:\My Projects\FrameWork Global\LLM Skills"

# Create symbolic links for each skill collection
Write-Host "`nCreating symbolic links..."

# 1. Anthropics Skills - create links for each individual skill
$anthropicsPath = Join-Path $skillsBasePath "anthropics-skills"
$anthropicsSkills = @(
    "algorithmic-art",
    "artifacts-builder",
    "brand-guidelines",
    "canvas-design",
    "document-skills",
    "internal-comms",
    "mcp-builder",
    "skill-creator",
    "slack-gif-creator",
    "template-skill",
    "theme-factory",
    "webapp-testing"
)

foreach ($skill in $anthropicsSkills) {
    $source = Join-Path $anthropicsPath $skill
    $target = Join-Path $claudeSkillsPath $skill
    if (Test-Path $source) {
        if (Test-Path $target) {
            Write-Host "  Removing existing: $skill"
            Remove-Item $target -Force -Recurse
        }
        Write-Host "  Linking: $skill (from anthropics-skills)"
        New-Item -ItemType SymbolicLink -Path $target -Target $source | Out-Null
    }
}

# 2. Custom Skills - create links for each skill file
$customPath = Join-Path $skillsBasePath "custom-skills"
$customSkills = @(
    @{Name="1c-bsl"; File="1C_BSL_SKILL.md"},
    @{Name="docker"; File="DOCKER_SKILLS.md"},
    @{Name="go"; File="GO_SKILL.md"},
    @{Name="mermaid"; File="MERMAID_SKILL.md"},
    @{Name="powershell"; File="POWERSHELL_RULES.md"},
    @{Name="skill-creator-v2"; File="USER_SKILL_RULE_V2.md"},
    @{Name="yaxunit"; File="YAXUNIT_TESTING_SKILL.md"}
)

foreach ($skill in $customSkills) {
    $skillDir = Join-Path $claudeSkillsPath $skill.Name
    $sourceFile = Join-Path $customPath $skill.File

    if (Test-Path $sourceFile) {
        # Create skill directory
        New-Item -ItemType Directory -Force -Path $skillDir | Out-Null

        # Create symlink to SKILL.md
        $targetFile = Join-Path $skillDir "SKILL.md"
        if (Test-Path $targetFile) {
            Remove-Item $targetFile -Force
        }
        Write-Host "  Linking: $($skill.Name) (from custom-skills)"
        New-Item -ItemType SymbolicLink -Path $targetFile -Target $sourceFile | Out-Null
    }
}

Write-Host ""
Write-Host "Skills setup complete!"
Write-Host ""
Write-Host "Skills are now available at: $claudeSkillsPath"
