wow.export is included only for code reference. Do not run it locally and do not modify its code from this repository.

- Commit hygiene: squash commits before merging to `main`.
  ```bash
  git fetch origin
  git rebase -i origin/main   # squash
  git push -f <your-branch>
  ```
- Documentation updates: always incorporate maintainer feedback into this file promptly. Keep it compact and high-signal (short bullets over long prose).