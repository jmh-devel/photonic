#!/bin/bash
# Photonic Library Reorganization Script
# Reorganizes photos from /data/Photography/Photography/ to /data/Photography/YYYY/MM/DD/ structure

set -e  # Exit on any error

# Configuration
SOURCE_DIR="/data/Photography/Photography"
TARGET_DIR="/data/Photography" 
DRY_RUN=true  # Set to false to actually move files
LOG_FILE="/tmp/photo_reorganization.log"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

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

# Function to get capture date from file using exiftool
get_capture_date() {
    local file="$1"
    local date_taken
    
    # Try different date fields in order of preference
    date_taken=$(exiftool -CreateDate -DateTimeOriginal -DateTime -FileCreateDate -d "%Y:%m:%d" -T -s -S -q "$file" 2>/dev/null | head -1)
    
    # If no EXIF date found, use file modification time
    if [[ -z "$date_taken" || "$date_taken" == "-" ]]; then
        date_taken=$(stat -c %y "$file" | cut -d' ' -f1 | tr '-' ':')
        warn "No EXIF date for $file, using file date: $date_taken"
    fi
    
    echo "$date_taken"
}

# Function to get target directory based on date
get_target_dir() {
    local date_taken="$1"
    local year=$(echo "$date_taken" | cut -d: -f1)
    local month=$(echo "$date_taken" | cut -d: -f2)
    local day=$(echo "$date_taken" | cut -d: -f3)
    
    # Validate date components
    if [[ ! "$year" =~ ^[0-9]{4}$ ]] || [[ ! "$month" =~ ^[0-9]{2}$ ]] || [[ ! "$day" =~ ^[0-9]{2}$ ]]; then
        error "Invalid date format: $date_taken"
        return 1
    fi
    
    echo "$TARGET_DIR/$year/$month/$day"
}

# Function to move file with conflict resolution
move_file() {
    local source_file="$1"
    local target_dir="$2"
    local filename=$(basename "$source_file")
    local target_file="$target_dir/$filename"
    
    # Create target directory
    if [[ "$DRY_RUN" == "false" ]]; then
        mkdir -p "$target_dir"
    else
        log "DRY RUN: Would create directory $target_dir"
    fi
    
    # Handle file conflicts
    if [[ -f "$target_file" ]]; then
        local source_hash=$(md5sum "$source_file" | cut -d' ' -f1)
        local target_hash=$(md5sum "$target_file" | cut -d' ' -f1)
        
        if [[ "$source_hash" == "$target_hash" ]]; then
            log "Duplicate file detected: $filename (keeping target, removing source)"
            if [[ "$DRY_RUN" == "false" ]]; then
                rm "$source_file"
            else
                log "DRY RUN: Would remove duplicate $source_file"
            fi
            return 0
        else
            # Files are different, create unique name
            local base_name="${filename%.*}"
            local extension="${filename##*.}"
            local counter=1
            
            while [[ -f "$target_dir/${base_name}_${counter}.${extension}" ]]; do
                ((counter++))
            done
            
            target_file="$target_dir/${base_name}_${counter}.${extension}"
            warn "File conflict: renamed to ${base_name}_${counter}.${extension}"
        fi
    fi
    
    # Move the file
    if [[ "$DRY_RUN" == "false" ]]; then
        mv "$source_file" "$target_file"
        success "Moved: $source_file → $target_file"
    else
        log "DRY RUN: Would move $source_file → $target_file"
    fi
}

# Function to process XMP sidecar files
move_sidecar() {
    local source_file="$1"
    local target_dir="$2"
    local base_name="${source_file%.*}"
    
    # Look for associated files
    for ext in xmp XMP xml; do
        local sidecar="${base_name}.${ext}"
        if [[ -f "$sidecar" ]]; then
            local sidecar_target="$target_dir/$(basename "$sidecar")"
            if [[ "$DRY_RUN" == "false" ]]; then
                mv "$sidecar" "$sidecar_target"
                log "Moved sidecar: $sidecar → $sidecar_target"
            else
                log "DRY RUN: Would move sidecar $sidecar → $sidecar_target"
            fi
        fi
    done
}

# Main reorganization function
reorganize_photos() {
    local total_files=0
    local processed_files=0
    local error_files=0
    
    log "Starting photo reorganization..."
    log "Source: $SOURCE_DIR"
    log "Target: $TARGET_DIR" 
    log "Dry run: $DRY_RUN"
    
    # Count total files first
    log "Counting files..."
    total_files=$(find "$SOURCE_DIR" -type f \( -iname "*.cr2" -o -iname "*.nef" -o -iname "*.dng" -o -iname "*.jpg" -o -iname "*.jpeg" -o -iname "*.tif" -o -iname "*.tiff" -o -iname "*.png" \) | wc -l)
    log "Found $total_files photo files to process"
    
    # Process each photo file
    find "$SOURCE_DIR" -type f \( -iname "*.cr2" -o -iname "*.nef" -o -iname "*.dng" -o -iname "*.jpg" -o -iname "*.jpeg" -o -iname "*.tif" -o -iname "*.tiff" -o -iname "*.png" \) | while IFS= read -r file; do
        ((processed_files++))
        echo -ne "\rProgress: $processed_files/$total_files ($(( processed_files * 100 / total_files ))%)"
        
        # Get capture date
        date_taken=$(get_capture_date "$file")
        if [[ $? -ne 0 ]]; then
            ((error_files++))
            continue
        fi
        
        # Get target directory
        target_dir=$(get_target_dir "$date_taken")
        if [[ $? -ne 0 ]]; then
            ((error_files++))
            continue
        fi
        
        # Move file and sidecars
        if move_file "$file" "$target_dir"; then
            move_sidecar "$file" "$target_dir"
        else
            ((error_files++))
        fi
    done
    
    echo  # New line after progress
    success "Reorganization complete!"
    log "Total files: $total_files"
    log "Processed: $processed_files"
    log "Errors: $error_files"
}

# Function to clean up empty directories
cleanup_empty_dirs() {
    log "Cleaning up empty directories in $SOURCE_DIR..."
    if [[ "$DRY_RUN" == "false" ]]; then
        find "$SOURCE_DIR" -type d -empty -delete
        success "Empty directories removed"
    else
        local empty_dirs=$(find "$SOURCE_DIR" -type d -empty | wc -l)
        log "DRY RUN: Would remove $empty_dirs empty directories"
    fi
}

# Function to generate summary report
generate_report() {
    log "Generating organization report..."
    echo "=== PHOTO ORGANIZATION REPORT ===" > /tmp/photo_report.txt
    echo "Date: $(date)" >> /tmp/photo_report.txt
    echo "Source: $SOURCE_DIR" >> /tmp/photo_report.txt
    echo "Target: $TARGET_DIR" >> /tmp/photo_report.txt
    echo "" >> /tmp/photo_report.txt
    
    # Count files by year
    echo "Files by year:" >> /tmp/photo_report.txt
    for year_dir in "$TARGET_DIR"/[0-9][0-9][0-9][0-9]; do
        if [[ -d "$year_dir" ]]; then
            local year=$(basename "$year_dir")
            local count=$(find "$year_dir" -type f \( -iname "*.cr2" -o -iname "*.nef" -o -iname "*.dng" -o -iname "*.jpg" -o -iname "*.jpeg" \) | wc -l)
            echo "  $year: $count files" >> /tmp/photo_report.txt
        fi
    done
    
    success "Report saved to /tmp/photo_report.txt"
}

# Main execution
main() {
    log "=== PHOTONIC LIBRARY REORGANIZATION ==="
    
    # Check dependencies
    if ! command -v exiftool &> /dev/null; then
        error "exiftool is required but not installed. Please install it first:"
        error "  sudo apt install exiftool"
        exit 1
    fi
    
    # Check source directory exists
    if [[ ! -d "$SOURCE_DIR" ]]; then
        error "Source directory does not exist: $SOURCE_DIR"
        exit 1
    fi
    
    # Create target directory
    if [[ "$DRY_RUN" == "false" ]]; then
        mkdir -p "$TARGET_DIR"
    fi
    
    # Run reorganization
    reorganize_photos
    cleanup_empty_dirs
    generate_report
    
    log "=== REORGANIZATION COMPLETE ==="
    if [[ "$DRY_RUN" == "true" ]]; then
        warn "This was a DRY RUN. Set DRY_RUN=false to actually move files."
        warn "Review the log at $LOG_FILE before proceeding."
    fi
}

# Parse command line arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        --no-dry-run)
            DRY_RUN=false
            shift
            ;;
        --source)
            SOURCE_DIR="$2"
            shift 2
            ;;
        --target) 
            TARGET_DIR="$2"
            shift 2
            ;;
        -h|--help)
            echo "Usage: $0 [OPTIONS]"
            echo "Options:"
            echo "  --no-dry-run    Actually move files (default: dry run)"
            echo "  --source DIR    Source directory (default: /data/Photography/Photography)"
            echo "  --target DIR    Target directory (default: /data/Photography)"
            echo "  -h, --help      Show this help"
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