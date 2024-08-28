#!/bin/bash

# Navigate to your project directory
cd ~/Documents/Projects/ccwig

# Check for changes
if [[ $(git status --porcelain) ]]; then
  # Add changes to the staging area
  git add .

  # Commit changes
  git commit -m "Schedled Commit on $(date)"

  # Push changes to the remote repository
  git pull
  git push
else
  echo "No New changes to commit."
fi
