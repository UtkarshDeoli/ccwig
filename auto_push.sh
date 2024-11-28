#!/bin/bash

# Check for changes
if [[ $(git status --porcelain) ]]; then
  # Add changes to the staging area
  git add .

  # Commit changes
  git commit -m "Auto Push with changes $(date)"

  # Push changes to the remote repository
  git pull
  git push
else
  echo "No New changes to commit."
fi
