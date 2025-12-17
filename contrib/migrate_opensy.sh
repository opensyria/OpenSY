#!/bin/bash
# migrate_opensy.sh - Safe migration from old data directory to .opensy
#
# Copyright (c) 2025 The OpenSY developers
# Distributed under the MIT software license
#
# This script safely migrates existing data directories (if any used an older
# naming convention) to the current .opensy directory structure.
#
# NOTE: The domain opensyria.net is intentionally used (opensy.net unavailable).
# The product name is OpenSY, but URLs use opensyria.net.

set -euo pipefail

# Color codes for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

echo -e "${BLUE}=========================================${NC}"
echo -e "${BLUE}  OpenSY Data Directory Migration Tool  ${NC}"
echo -e "${BLUE}  (Product: OpenSY | Domain: opensyria.net)${NC}"
echo -e "${BLUE}=========================================${NC}"
echo ""

# Determine OS-specific paths
case "$(uname -s)" in
    Darwin)
        OLD_DIR="$HOME/Library/Application Support/OpenSyria"
        NEW_DIR="$HOME/Library/Application Support/OpenSY"
        ;;
    Linux)
        OLD_DIR="$HOME/.openSyria"
        NEW_DIR="$HOME/.opensy"
        ;;
    CYGWIN*|MINGW*|MSYS*)
        OLD_DIR="$APPDATA/OpenSyria"
        NEW_DIR="$APPDATA/OpenSY"
        ;;
    *)
        echo -e "${RED}ERROR: Unsupported operating system${NC}"
        exit 1
        ;;
esac

# Also check lowercase variant for Linux
if [[ "$(uname -s)" == "Linux" ]]; then
    ALT_OLD_DIR="$HOME/.opensyria"
else
    ALT_OLD_DIR=""
fi

echo "Checking for existing data directories..."
echo ""

# Function to migrate a directory
migrate_directory() {
    local source="$1"
    local dest="$2"
    
    echo -e "${YELLOW}Source:${NC} $source"
    echo -e "${YELLOW}Destination:${NC} $dest"
    
    # Calculate directory size
    if command -v du &> /dev/null; then
        SIZE=$(du -sh "$source" 2>/dev/null | cut -f1)
        echo -e "${YELLOW}Data size:${NC} $SIZE"
    fi
    
    # Count blocks
    if [ -f "$source/blocks/blk00000.dat" ]; then
        BLOCK_COUNT=$(find "$source/blocks" -name "blk*.dat" 2>/dev/null | wc -l | tr -d ' ')
        echo -e "${YELLOW}Block files:${NC} $BLOCK_COUNT"
    fi
    
    echo ""
    echo -e "${YELLOW}WARNING: This will move your blockchain data.${NC}"
    echo "A backup marker will be created, but no actual backup is made."
    echo ""
    
    read -p "Proceed with migration? (y/N) " -n 1 -r
    echo
    
    if [[ $REPLY =~ ^[Yy]$ ]]; then
        echo ""
        echo "Step 1: Creating backup marker..."
        BACKUP_MARKER="$source/.migration_backup_$(date +%Y%m%d_%H%M%S)"
        touch "$BACKUP_MARKER"
        echo -e "${GREEN}Created:${NC} $BACKUP_MARKER"
        
        echo ""
        echo "Step 2: Moving data directory..."
        mv "$source" "$dest"
        echo -e "${GREEN}Moved:${NC} $source -> $dest"
        
        echo ""
        echo "Step 3: Creating backward compatibility symlink..."
        ln -s "$dest" "$source"
        echo -e "${GREEN}Symlink:${NC} $source -> $dest"
        
        echo ""
        echo -e "${GREEN}=========================================${NC}"
        echo -e "${GREEN}  Migration completed successfully!      ${NC}"
        echo -e "${GREEN}=========================================${NC}"
        echo ""
        echo "Your data is now at: $dest"
        echo "The old path still works via symlink."
        echo ""
        echo "You can now start opensyd or opensy-qt."
        
        return 0
    else
        echo ""
        echo -e "${YELLOW}Migration cancelled by user.${NC}"
        return 1
    fi
}

# Check for existing directories
FOUND_OLD=false
FOUND_NEW=false

if [ -d "$OLD_DIR" ] && [ ! -L "$OLD_DIR" ]; then
    FOUND_OLD=true
    echo -e "${GREEN}Found:${NC} $OLD_DIR"
fi

if [ -n "$ALT_OLD_DIR" ] && [ -d "$ALT_OLD_DIR" ] && [ ! -L "$ALT_OLD_DIR" ]; then
    FOUND_OLD=true
    OLD_DIR="$ALT_OLD_DIR"
    echo -e "${GREEN}Found:${NC} $ALT_OLD_DIR"
fi

if [ -d "$NEW_DIR" ]; then
    FOUND_NEW=true
    echo -e "${GREEN}Found:${NC} $NEW_DIR (new location)"
fi

echo ""

# Decision logic
if $FOUND_OLD && $FOUND_NEW; then
    echo -e "${RED}ERROR: Both old and new directories exist!${NC}"
    echo ""
    echo "Old: $OLD_DIR"
    echo "New: $NEW_DIR"
    echo ""
    echo "This could mean:"
    echo "1. A partial migration was interrupted"
    echo "2. You have two separate installations"
    echo ""
    echo "Please manually inspect and resolve before proceeding."
    echo "If the new directory is empty, remove it and run this script again."
    exit 1
elif $FOUND_OLD; then
    echo "Old data directory found."
    echo "This will be migrated to the current .opensy location."
    echo ""
    migrate_directory "$OLD_DIR" "$NEW_DIR"
elif $FOUND_NEW; then
    echo -e "${GREEN}Already using new OpenSY directory structure.${NC}"
    echo "Location: $NEW_DIR"
    echo ""
    echo "No migration needed."
    exit 0
else
    echo "No existing data directory found."
    echo ""
    echo "Fresh installations will automatically use:"
    echo "  $NEW_DIR"
    echo ""
    echo "No migration needed."
    exit 0
fi
