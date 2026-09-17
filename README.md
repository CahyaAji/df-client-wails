# README

## About

This is the official Wails Svelte-TS template.

## Live Development

wails dev -tags webkit2_41

wails build -tags webkit2_41


## merge dev to main
```
# 1. Make sure your dev branch is committed and clean
git status
git add .
git commit -m "finish feature"

# 2. Switch to main and update it
git checkout main
git pull origin main   # if working with a remote, get latest changes

# 3. Merge dev into main
git merge dev

# 4. Push the updated main
git push origin main
```

check merge dengan
```
git diff main dev
# No output = identical content.
git log --oneline --graph --all -10
```