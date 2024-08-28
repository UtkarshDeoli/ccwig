#!/bin/bash

# Navigate to your project directory
cd ~/Documents/Projects/ccwig

# Check for changes
if [[ $(git status --porcelain) ]]; then
  # Add changes to the staging area
  git add .

  # Commit changes
  git commit -m "Automated commit on $(date)"

  # Push changes to the remote repository
  git push
else
  echo "No changes to commit."
fi