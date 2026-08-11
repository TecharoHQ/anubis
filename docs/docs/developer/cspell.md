# Spellchecking

Anubis uses [CSpell](https://cspell.org/) to ensure words are spelled correctly. The project dictionary is at `.vscode/project-words.txt`.

## Manually testing spellchecking

To manually run spellchecking against the files you have modified before commit:

```sh
npm run test:spelling
```

## Adding words to the dictionary

To add a word to the project dictionary (eg: `${WORD}`), do the following:

### Append the word to the dictionary

```sh
echo ${WORD} >> .vscode/project-words.txt
```

### Sort the dictionary list

```sh
npm run test:spelling:sort
```

### Commit the results

```sh
git add .vscode/project-words.txt
git commit -sm "chore: spelling"
```
