#!/bin/bash

# Function to read commit message with a timeout
read_commit_message() {
  read -t 60 -p "Enter commit message: " commit_message
  if [ -z "$commit_message" ]; then
    commit_message="Auto Push with changes $(date)"
  fi
}

# Check for changes
if [[ $(git status --porcelain) ]]; then
  # Add changes to the staging area
  git add .

  # Get commit message
  read_commit_message

  # Commit changes
  git commit -m "$commit_message"

  # Push changes to the remote repository
  git pull
  git push
else
  echo "No New changes to commit."
fi
