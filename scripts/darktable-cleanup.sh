#!/bin/bash
# Darktable Library Cleanup and Re-import Script
# Safely cleans darktable library and re-imports photos from organized structure

set -e

# Configuration
PHOTO_ROOT="/data/Photography"
DARKTABLE_CONFIG_DIR="$HOME/.config/darktable"
DRY_RUN=true
LOG_FILE="/tmp/darktable_cleanup.log"

# Colors for output  
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

log() {
    echo -e "${BLUE}[$(date '+%Y-%m-%d %H:%M:%S')]${NC} $1" | tee -a "$LOG_FILE"
}

error() {
    echo -e "${RED}[ERROR]${NC} $1" | tee -a "$LOG_FILE"
}

warn() {
    echo -e "${YELLOW}[WARN]${NC} $1" | tee -a "$LOG_FILE"
}

success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1" | tee -a "$LOG_FILE"
}

# Function to backup darktable database
backup_darktable() {
    local backup_dir="$DARKTABLE_CONFIG_DIR/backup_$(date +%Y%m%d_%H%M%S)"
    
    log "Creating darktable backup at $backup_dir"
    
    if [[ "$DRY_RUN" == "false" ]]; then
        mkdir -p "$backup_dir"
        cp "$DARKTABLE_CONFIG_DIR/library.db" "$backup_dir/"
        cp "$DARKTABLE_CONFIG_DIR/data.db" "$backup_dir/"
        
        # Backup presets and styles if they exist
        if [[ -d "$DARKTABLE_CONFIG_DIR/styles" ]]; then
            cp -r "$DARKTABLE_CONFIG_DIR/styles" "$backup_dir/"
        fi
        
        if [[ -d "$DARKTABLE_CONFIG_DIR/presets" ]]; then
            cp -r "$DARKTABLE_CONFIG_DIR/presets" "$backup_dir/"
        fi
        
        success "Darktable backup created at $backup_dir"
    else
        log "DRY RUN: Would create backup at $backup_dir"
    fi
}

# Function to export current darktable library info
export_library_info() {
    log "Exporting current library information..."
    
    # Check if darktable is running
    if pgrep -x "darktable" > /dev/null; then
        warn "Darktable is currently running. Please close it before proceeding."
        return 1
    fi
    
    # Export library statistics using sqlite3
    if command -v sqlite3 &> /dev/null; then
        local stats_file="/tmp/darktable_library_stats.txt"
        
        echo "=== DARKTABLE LIBRARY STATISTICS ===" > "$stats_file"
        echo "Export Date: $(date)" >> "$stats_file"
        echo "" >> "$stats_file"
        
        # Count total images
        local total_images=$(sqlite3 "$DARKTABLE_CONFIG_DIR/library.db" "SELECT COUNT(*) FROM main.images;")
        echo "Total images in library: $total_images" >> "$stats_file"
        
        # Count by file type
        echo "" >> "$stats_file"
        echo "Files by extension:" >> "$stats_file"
        sqlite3 "$DARKTABLE_CONFIG_DIR/library.db" "
        SELECT 
            UPPER(SUBSTR(filename, LENGTH(filename) - INSTR(REVERSE(filename), '.') + 2)) as extension,
            COUNT(*) as count
        FROM main.images 
        GROUP BY extension 
        ORDER BY count DESC;
        " >> "$stats_file"
        
        # Count files with ratings
        echo "" >> "$stats_file"
        echo "Files with ratings:" >> "$stats_file"
        sqlite3 "$DARKTABLE_CONFIG_DIR/library.db" "
        SELECT 
            flags & 7 as rating,
            COUNT(*) as count
        FROM main.images 
        WHERE (flags & 7) > 0
        GROUP BY rating 
        ORDER BY rating;
        " >> "$stats_file"
        
        # Count files with color labels
        echo "" >> "$stats_file"
        echo "Files with color labels:" >> "$stats_file"
        sqlite3 "$DARKTABLE_CONFIG_DIR/library.db" "
        SELECT 
            (flags >> 8) & 7 as color_label,
            COUNT(*) as count
        FROM main.images 
        WHERE ((flags >> 8) & 7) > 0
        GROUP BY color_label;
        " >> "$stats_file"
        
        success "Library statistics exported to $stats_file"
    else
        warn "sqlite3 not available, skipping library statistics export"
    fi
}

# Function to clean darktable library  
clean_darktable_library() {
    log "Cleaning darktable library (removing all image records)..."
    
    if [[ "$DRY_RUN" == "false" ]]; then
        # Make sure darktable is not running
        if pgrep -x "darktable" > /dev/null; then
            error "Darktable is running. Please close it first."
            return 1
        fi
        
        # Clean the library database
        sqlite3 "$DARKTABLE_CONFIG_DIR/library.db" "
        DELETE FROM main.images;
        DELETE FROM main.selected_images;
        DELETE FROM main.history;
        DELETE FROM main.mask;
        DELETE FROM main.tagged_images;
        DELETE FROM main.used_tags;
        DELETE FROM main.color_labels;
        VACUUM;
        "
        
        success "Darktable library cleaned (all image records removed)"
        log "Presets, styles, and settings were preserved"
    else
        log "DRY RUN: Would clean darktable library database"
    fi
}

# Function to import photos back into darktable
import_photos() {
    log "Starting darktable import process..."
    
    if [[ ! -d "$PHOTO_ROOT" ]]; then
        error "Photo root directory not found: $PHOTO_ROOT"
        return 1
    fi
    
    # Count total photos to import
    local total_photos=$(find "$PHOTO_ROOT" -type f \( -iname "*.cr2" -o -iname "*.nef" -o -iname "*.dng" -o -iname "*.jpg" -o -iname "*.jpeg" -o -iname "*.tif" -o -iname "*.tiff" \) | wc -l)
    log "Found $total_photos photos to import"
    
    if [[ "$DRY_RUN" == "false" ]]; then
        # Import each year directory separately to avoid overwhelming darktable
        for year_dir in "$PHOTO_ROOT"/[0-9][0-9][0-9][0-9]; do
            if [[ -d "$year_dir" ]]; then
                local year=$(basename "$year_dir")
                local year_photos=$(find "$year_dir" -type f \( -iname "*.cr2" -o -iname "*.nef" -o -iname "*.dng" -o -iname "*.jpg" -o -iname "*.jpeg" \) | wc -l)
                
                if [[ $year_photos -gt 0 ]]; then
                    log "Importing $year_photos photos from $year..."
                    
                    # Use darktable-cli to import (non-interactive)
                    # Note: This will automatically detect and import all supported files
                    darktable --library "$DARKTABLE_CONFIG_DIR/library.db" --import "$year_dir" --recursive
                    
                    success "Imported $year ($year_photos photos)"
                fi
            fi
        done
        
        success "Photo import completed"
    else
        log "DRY RUN: Would import $total_photos photos using darktable --import"
        
        # Show what would be imported
        for year_dir in "$PHOTO_ROOT"/[0-9][0-9][0-9][0-9]; do
            if [[ -d "$year_dir" ]]; then
                local year=$(basename "$year_dir")
                local year_photos=$(find "$year_dir" -type f \( -iname "*.cr2" -o -iname "*.nef" -o -iname "*.dng" -o -iname "*.jpg" -o -iname "*.jpeg" \) | wc -l)
                if [[ $year_photos -gt 0 ]]; then
                    log "DRY RUN: Would import $year ($year_photos photos)"
                fi
            fi
        done
    fi
}

# Function to optimize darktable database after import
optimize_database() {
    log "Optimizing darktable database..."
    
    if [[ "$DRY_RUN" == "false" ]]; then
        sqlite3 "$DARKTABLE_CONFIG_DIR/library.db" "
        ANALYZE;
        VACUUM;
        "
        success "Database optimized"
    else
        log "DRY RUN: Would optimize database"
    fi
}

# Function to create import summary
create_import_summary() {
    log "Creating import summary..."
    
    local summary_file="/tmp/darktable_import_summary.txt"
    
    echo "=== DARKTABLE IMPORT SUMMARY ===" > "$summary_file"
    echo "Import Date: $(date)" >> "$summary_file"
    echo "Photo Root: $PHOTO_ROOT" >> "$summary_file"
    echo "" >> "$summary_file"
    
    if [[ "$DRY_RUN" == "false" ]]; then
        # Get post-import statistics
        local total_imported=$(sqlite3 "$DARKTABLE_CONFIG_DIR/library.db" "SELECT COUNT(*) FROM main.images;")
        echo "Total images imported: $total_imported" >> "$summary_file"
        
        echo "" >> "$summary_file"
        echo "Images by year:" >> "$summary_file"
        
        for year_dir in "$PHOTO_ROOT"/[0-9][0-9][0-9][0-9]; do
            if [[ -d "$year_dir" ]]; then
                local year=$(basename "$year_dir")
                local year_photos=$(find "$year_dir" -type f \( -iname "*.cr2" -o -iname "*.nef" -o -iname "*.dng" -o -iname "*.jpg" -o -iname "*.jpeg" \) | wc -l)
                echo "  $year: $year_photos files" >> "$summary_file"
            fi
        done
    else
        echo "DRY RUN - No actual import performed" >> "$summary_file"
    fi
    
    success "Import summary saved to $summary_file"
}

# Main execution function
main() {
    log "=== DARKTABLE LIBRARY CLEANUP & REORGANIZATION ==="
    
    # Check dependencies
    if ! command -v darktable &> /dev/null; then
        error "Darktable is not installed or not in PATH"
        exit 1
    fi
    
    if ! command -v sqlite3 &> /dev/null; then
        warn "sqlite3 not available - some features will be limited"
    fi
    
    # Check if darktable config exists
    if [[ ! -f "$DARKTABLE_CONFIG_DIR/library.db" ]]; then
        error "Darktable library database not found: $DARKTABLE_CONFIG_DIR/library.db"
        exit 1
    fi
    
    # Check if photo root exists
    if [[ ! -d "$PHOTO_ROOT" ]]; then
        error "Photo root directory not found: $PHOTO_ROOT"
        exit 1
    fi
    
    # Execute cleanup process
    backup_darktable
    export_library_info
    clean_darktable_library
    import_photos
    optimize_database
    create_import_summary
    
    log "=== CLEANUP & IMPORT COMPLETE ==="
    
    if [[ "$DRY_RUN" == "true" ]]; then
        warn "This was a DRY RUN. Set DRY_RUN=false to actually perform operations."
        warn "Review logs before proceeding with actual cleanup."
    else
        success "Darktable library has been cleaned and reorganized!"
        success "You can now open darktable to see your newly organized library."
    fi
}

# Parse command line arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        --no-dry-run)
            DRY_RUN=false
            shift
            ;;
        --photo-root)
            PHOTO_ROOT="$2"
            shift 2
            ;;
        -h|--help)
            echo "Usage: $0 [OPTIONS]"
            echo "Options:"
            echo "  --no-dry-run      Actually perform operations (default: dry run)"
            echo "  --photo-root DIR  Photo root directory (default: /data/Photography)"
            echo "  -h, --help        Show this help"
            echo ""
            echo "This script will:"
            echo "  1. Backup your darktable database"
            echo "  2. Export current library statistics"
            echo "  3. Clean the darktable library (remove all image records)"
            echo "  4. Re-import photos from organized directory structure"
            echo "  5. Optimize the database"
            exit 0
            ;;
        *)
            error "Unknown option: $1"
            exit 1
            ;;
    esac
done

# Run main function
main "$@"