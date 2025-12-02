#!/usr/bin/env python3
"""
JMH Photo Consolidation Script
=============================

Consolidates photos from multiple sources, deduplicates, and organizes them
into a clean darktable-compatible structure.

Usage:
    python3 jmh_consolidation.py [--dry-run] [--stage-dir /path/to/staging]

Author: Photonic Team
Date: November 30, 2025
"""

import os
import sys
import subprocess
import shutil
import hashlib
import logging
from pathlib import Path
from datetime import datetime
from collections import defaultdict
import argparse
import json
import time

# Configure logging
logging.basicConfig(
    level=logging.INFO,
    format='%(asctime)s [%(levelname)s] %(message)s',
    handlers=[
        logging.FileHandler('/tmp/photo_consolidation.log'),
        logging.StreamHandler()
    ]
)
logger = logging.getLogger(__name__)

class PhotoConsolidator:
    """Handles the complete photo consolidation workflow."""
    
    def __init__(self, stage_dir="/mnt/staging", dry_run=False):
        self.stage_dir = Path(stage_dir)
        self.dry_run = dry_run
        self.photo_extensions = {
            '.cr2', '.raw', '.dng', '.nef', '.arw', '.orf', '.rw2',
            '.jpg', '.jpeg', '.tif', '.tiff', '.png', '.bmp'
        }
        self.video_extensions = {
            '.mp4', '.mov', '.avi', '.mkv', '.m4v', '.3gp'
        }
        
        # Statistics tracking
        self.stats = {
            'files_found': 0,
            'duplicates_found': 0,
            'files_copied': 0,
            'total_size_mb': 0,
            'errors': []
        }
    
    def discover_photo_sources(self):
        """Discover all photo sources on the system."""
        logger.info("🔍 Discovering photo sources...")
        
        sources = {
            'primary': Path('/data/Photography'),
            'media_drive': Path('/media/joel'),
            'external': Path('/mnt/external'),
        }
        
        discovered = {}
        
        for name, path in sources.items():
            if path.exists():
                logger.info(f"📁 Scanning {name}: {path}")
                file_count = self._count_photos_in_path(path)
                size_mb = self._get_directory_size_mb(path)
                
                discovered[name] = {
                    'path': str(path),
                    'file_count': file_count,
                    'size_mb': size_mb
                }
                
                logger.info(f"   Found {file_count:,} photos ({size_mb:,.0f} MB)")
            else:
                logger.warning(f"⚠️  Source not found: {path}")
        
        return discovered
    
    def _count_photos_in_path(self, path):
        """Count photo files in a directory tree."""
        count = 0
        try:
            for root, dirs, files in os.walk(path):
                for file in files:
                    if Path(file).suffix.lower() in self.photo_extensions:
                        count += 1
        except PermissionError as e:
            logger.warning(f"Permission denied: {e}")
        
        return count
    
    def _get_directory_size_mb(self, path):
        """Get total directory size in MB."""
        total_size = 0
        try:
            for root, dirs, files in os.walk(path):
                for file in files:
                    try:
                        file_path = Path(root) / file
                        total_size += file_path.stat().st_size
                    except (OSError, FileNotFoundError):
                        continue
        except PermissionError:
            pass
        
        return total_size / (1024 * 1024)  # Convert to MB
    
    def prepare_staging_area(self):
        """Prepare the staging directory."""
        logger.info(f"🗂️  Preparing staging area: {self.stage_dir}")
        
        if not self.dry_run:
            self.stage_dir.mkdir(parents=True, exist_ok=True)
            
            # Create subdirectories
            (self.stage_dir / "raw_photos").mkdir(exist_ok=True)
            (self.stage_dir / "processed_photos").mkdir(exist_ok=True) 
            (self.stage_dir / "videos").mkdir(exist_ok=True)
            (self.stage_dir / "duplicates").mkdir(exist_ok=True)
    
    def gather_all_photos(self, sources):
        """Gather all photos into staging area."""
        logger.info("📦 Gathering photos from all sources...")
        
        for source_name, source_info in sources.items():
            source_path = Path(source_info['path'])
            
            logger.info(f"   Processing {source_name}: {source_path}")
            
            self._copy_photos_from_source(source_path, source_name)
    
    def _copy_photos_from_source(self, source_path, source_name):
        """Copy photos from a single source to staging."""
        copied = 0
        
        try:
            for root, dirs, files in os.walk(source_path):
                for file in files:
                    file_path = Path(root) / file
                    extension = file_path.suffix.lower()
                    
                    if extension in self.photo_extensions:
                        dest_subdir = "raw_photos" if extension in {'.cr2', '.raw', '.dng', '.nef', '.arw', '.orf', '.rw2'} else "processed_photos"
                    elif extension in self.video_extensions:
                        dest_subdir = "videos"
                    else:
                        continue
                    
                    # Create unique filename to avoid conflicts
                    dest_file = f"{source_name}_{file}"
                    dest_path = self.stage_dir / dest_subdir / dest_file
                    
                    try:
                        if not self.dry_run:
                            shutil.copy2(file_path, dest_path)
                        
                        copied += 1
                        self.stats['files_copied'] += 1
                        
                        if copied % 100 == 0:
                            logger.info(f"   Copied {copied:,} files from {source_name}...")
                            
                    except Exception as e:
                        error_msg = f"Failed to copy {file_path}: {e}"
                        logger.error(error_msg)
                        self.stats['errors'].append(error_msg)
        
        except PermissionError as e:
            logger.warning(f"Permission denied scanning {source_path}: {e}")
        
        logger.info(f"✅ Copied {copied:,} files from {source_name}")
    
    def run_jdupes_deduplication(self):
        """Run jdupes to find and handle duplicates."""
        logger.info("🔍 Running jdupes deduplication...")
        
        if self.dry_run:
            logger.info("   (Dry run - skipping actual deduplication)")
            return
        
        # Run jdupes on the staging area
        cmd = [
            'jdupes', 
            '-r',           # Recursive
            '-A',           # No hidden files
            '-q',           # Quiet
            '-M',           # Print machine-readable output
            str(self.stage_dir / "raw_photos"),
            str(self.stage_dir / "processed_photos")
        ]
        
        try:
            logger.info(f"   Running: {' '.join(cmd)}")
            result = subprocess.run(cmd, capture_output=True, text=True, timeout=3600)
            
            if result.returncode == 0:
                # Parse jdupes output to count duplicates
                duplicate_groups = result.stdout.strip().split('\n\n')
                total_duplicates = sum(len(group.split('\n')) - 1 for group in duplicate_groups if group.strip())
                
                self.stats['duplicates_found'] = total_duplicates
                logger.info(f"✅ Found {total_duplicates:,} duplicate files")
                
                # Move duplicates to separate folder
                self._handle_duplicates(result.stdout)
                
            else:
                error_msg = f"jdupes failed: {result.stderr}"
                logger.error(error_msg)
                self.stats['errors'].append(error_msg)
                
        except subprocess.TimeoutExpired:
            error_msg = "jdupes timed out after 1 hour"
            logger.error(error_msg)
            self.stats['errors'].append(error_msg)
        except FileNotFoundError:
            error_msg = "jdupes not installed. Install with: sudo apt install jdupes"
            logger.error(error_msg)
            self.stats['errors'].append(error_msg)
    
    def _handle_duplicates(self, jdupes_output):
        """Move duplicate files to duplicates folder."""
        logger.info("📦 Moving duplicates to separate folder...")
        
        duplicate_groups = jdupes_output.strip().split('\n\n')
        moved = 0
        
        for group in duplicate_groups:
            if not group.strip():
                continue
                
            files = group.strip().split('\n')
            if len(files) < 2:
                continue
            
            # Keep the first file, move the rest
            for duplicate_file in files[1:]:
                duplicate_path = Path(duplicate_file.strip())
                if duplicate_path.exists():
                    try:
                        dest_path = self.stage_dir / "duplicates" / duplicate_path.name
                        shutil.move(str(duplicate_path), str(dest_path))
                        moved += 1
                    except Exception as e:
                        logger.error(f"Failed to move duplicate {duplicate_path}: {e}")
        
        logger.info(f"✅ Moved {moved:,} duplicate files")
    
    def organize_by_date(self):
        """Organize photos by YYYY/MM/DD structure using EXIF data."""
        logger.info("📅 Organizing photos by date...")
        
        if self.dry_run:
            logger.info("   (Dry run - skipping organization)")
            return
        
        organized_dir = self.stage_dir / "organized"
        organized_dir.mkdir(exist_ok=True)
        
        photo_dirs = [
            self.stage_dir / "raw_photos",
            self.stage_dir / "processed_photos"
        ]
        
        organized_count = 0
        
        for photo_dir in photo_dirs:
            if not photo_dir.exists():
                continue
                
            for photo_file in photo_dir.iterdir():
                if photo_file.is_file():
                    try:
                        # Get date from EXIF or file modification time
                        photo_date = self._get_photo_date(photo_file)
                        
                        # Create YYYY/MM/DD directory structure
                        year_dir = organized_dir / str(photo_date.year)
                        month_dir = year_dir / f"{photo_date.month:02d}"
                        day_dir = month_dir / f"{photo_date.day:02d}"
                        
                        day_dir.mkdir(parents=True, exist_ok=True)
                        
                        # Move photo to organized location
                        dest_path = day_dir / photo_file.name
                        shutil.move(str(photo_file), str(dest_path))
                        organized_count += 1
                        
                        if organized_count % 100 == 0:
                            logger.info(f"   Organized {organized_count:,} photos...")
                            
                    except Exception as e:
                        logger.error(f"Failed to organize {photo_file}: {e}")
        
        logger.info(f"✅ Organized {organized_count:,} photos by date")
    
    def _get_photo_date(self, photo_path):
        """Extract photo date from EXIF or use file modification time."""
        try:
            # Try to use exiftool to get the date
            cmd = ['exiftool', '-DateTimeOriginal', '-d', '%Y:%m:%d %H:%M:%S', str(photo_path)]
            result = subprocess.run(cmd, capture_output=True, text=True, timeout=10)
            
            if result.returncode == 0 and 'Date/Time Original' in result.stdout:
                date_line = result.stdout.strip()
                date_str = date_line.split(': ', 1)[1]
                return datetime.strptime(date_str, '%Y:%m:%d %H:%M:%S')
        
        except (subprocess.TimeoutExpired, FileNotFoundError, ValueError):
            pass
        
        # Fallback to file modification time
        return datetime.fromtimestamp(photo_path.stat().st_mtime)
    
    def import_to_darktable(self):
        """Import organized photos into darktable."""
        logger.info("📸 Importing photos into darktable...")
        
        if self.dry_run:
            logger.info("   (Dry run - skipping darktable import)")
            return
        
        organized_dir = self.stage_dir / "organized"
        final_photo_dir = Path("/data/Photography")
        
        if not organized_dir.exists():
            logger.error("Organized directory not found. Run organization step first.")
            return
        
        # Copy organized structure to final location
        try:
            logger.info(f"   Copying organized photos to {final_photo_dir}")
            
            for year_dir in organized_dir.iterdir():
                if year_dir.is_dir() and year_dir.name.isdigit():
                    dest_year_dir = final_photo_dir / year_dir.name
                    
                    if dest_year_dir.exists():
                        # Merge with existing year directory
                        self._merge_directories(year_dir, dest_year_dir)
                    else:
                        # Copy entire year directory
                        shutil.copytree(year_dir, dest_year_dir)
            
            logger.info("✅ Photos copied to final location")
            
            # Trigger darktable to scan for new images
            logger.info("   Triggering darktable database update...")
            
            # This assumes darktable CLI is available
            try:
                cmd = ['darktable-cli', '--library', '/data/Photography']
                subprocess.run(cmd, timeout=300)
                logger.info("✅ Darktable database updated")
            except (FileNotFoundError, subprocess.TimeoutExpired):
                logger.warning("Could not update darktable database automatically")
                
        except Exception as e:
            error_msg = f"Failed to import to darktable: {e}"
            logger.error(error_msg)
            self.stats['errors'].append(error_msg)
    
    def _merge_directories(self, src_dir, dest_dir):
        """Merge source directory into destination, handling conflicts."""
        for src_item in src_dir.iterdir():
            dest_item = dest_dir / src_item.name
            
            if src_item.is_dir():
                if dest_item.exists():
                    self._merge_directories(src_item, dest_item)
                else:
                    shutil.copytree(src_item, dest_item)
            else:
                if dest_item.exists():
                    # Handle file conflict - add suffix
                    counter = 1
                    base_name = dest_item.stem
                    extension = dest_item.suffix
                    
                    while dest_item.exists():
                        dest_item = dest_dir / f"{base_name}_{counter}{extension}"
                        counter += 1
                
                shutil.copy2(src_item, dest_item)
    
    def generate_report(self):
        """Generate a consolidation report."""
        logger.info("📊 Generating consolidation report...")
        
        report = {
            'consolidation_date': datetime.now().isoformat(),
            'statistics': self.stats,
            'dry_run': self.dry_run,
            'stage_directory': str(self.stage_dir)
        }
        
        report_file = Path('/tmp/photo_consolidation_report.json')
        
        with open(report_file, 'w') as f:
            json.dump(report, f, indent=2)
        
        logger.info(f"📋 Report saved to: {report_file}")
        logger.info("=" * 60)
        logger.info("CONSOLIDATION SUMMARY")
        logger.info("=" * 60)
        logger.info(f"Files found:        {self.stats['files_found']:,}")
        logger.info(f"Files copied:       {self.stats['files_copied']:,}")
        logger.info(f"Duplicates found:   {self.stats['duplicates_found']:,}")
        logger.info(f"Total size:         {self.stats['total_size_mb']:,.0f} MB")
        logger.info(f"Errors:             {len(self.stats['errors'])}")
        
        if self.stats['errors']:
            logger.info("ERRORS:")
            for error in self.stats['errors'][:10]:  # Show first 10 errors
                logger.info(f"  - {error}")
        
        return report
    
    def run_full_consolidation(self):
        """Run the complete consolidation workflow."""
        logger.info("🚀 Starting photo consolidation workflow...")
        start_time = time.time()
        
        try:
            # Step 1: Discover sources
            sources = self.discover_photo_sources()
            self.stats['files_found'] = sum(s['file_count'] for s in sources.values())
            self.stats['total_size_mb'] = sum(s['size_mb'] for s in sources.values())
            
            # Step 2: Prepare staging
            self.prepare_staging_area()
            
            # Step 3: Gather photos
            self.gather_all_photos(sources)
            
            # Step 4: Deduplicate
            self.run_jdupes_deduplication()
            
            # Step 5: Organize by date
            self.organize_by_date()
            
            # Step 6: Import to darktable
            self.import_to_darktable()
            
            # Step 7: Generate report
            self.generate_report()
            
            elapsed_time = time.time() - start_time
            logger.info(f"✅ Consolidation completed in {elapsed_time/60:.1f} minutes")
            
        except Exception as e:
            logger.error(f"💥 Consolidation failed: {e}")
            self.stats['errors'].append(str(e))
            raise


def main():
    parser = argparse.ArgumentParser(description='JMH Photo Consolidation')
    parser.add_argument('--dry-run', action='store_true', 
                       help='Perform a dry run without making changes')
    parser.add_argument('--stage-dir', default='/mnt/staging',
                       help='Directory for staging photos (default: /mnt/staging)')
    parser.add_argument('--step', choices=['discover', 'gather', 'dedupe', 'organize', 'import', 'all'],
                       default='all', help='Run specific step only')
    
    args = parser.parse_args()
    
    consolidator = PhotoConsolidator(stage_dir=args.stage_dir, dry_run=args.dry_run)
    
    if args.step == 'discover':
        sources = consolidator.discover_photo_sources()
        print(json.dumps(sources, indent=2))
    elif args.step == 'all':
        consolidator.run_full_consolidation()
    else:
        logger.error(f"Step '{args.step}' not implemented yet")


if __name__ == '__main__':
    main()