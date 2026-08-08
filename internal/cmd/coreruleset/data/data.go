package data

import "embed"

//go:embed *.txt
var Files embed.FS

//go:generate curl -qfSsL https://raw.githubusercontent.com/coreruleset/coreruleset/refs/heads/main/rules/ai-critical-artifacts.data -o ai-critical-artifacts.txt
//go:generate curl -qfSsL https://raw.githubusercontent.com/coreruleset/coreruleset/refs/heads/main/rules/lfi-os-files.data -o lfi-os-files.txt
//go:generate curl -qfSsL https://raw.githubusercontent.com/coreruleset/coreruleset/d694d465951ee06347214f1291ae096e40700001/rules/restricted-files.data -o restricted-files.txt
