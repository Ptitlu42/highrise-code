package cli

const usage_text = `Usage: highrise-code [options] <root>
       highrise-code --pick [options]

Visualize a project as an interactive treemap city: files are buildings,
directories are districts.

Options:
  -m, --mode iso|3d|2d          initial view (default 3d)
  -c, --color-by language|depth|size
                                building color (default language)
  -H, --height-by lines|code|size|flat
                                building height (default lines)
  -d, --max-depth N             maximum displayed depth (default unlimited, hard cap 256)
  -i, --ignore GLOB             ignore pattern, gitignore syntax, repeatable
      --include-hidden          include dotfiles and hidden directories
      --no-gitignore            do not apply .gitignore files
      --max-files N             maximum number of scanned files (default 2000000)
      --max-file-size SIZE      larger files are skipped and reported (default 64MiB)
      --workers N               scan workers (default number of CPUs, max 64)
      --port N                  local port (default: random free port)
      --no-open                 do not open the browser
      --pick                    choose the project folder in a small dialog
      --idle-timeout DURATION   stop the server after inactivity (default 0, disabled)
      --out FILE                write a standalone HTML file and exit ("-" for stdout)
      --stats                   print all statistics to the terminal and exit
      --format text|json        format used by --stats (default text)
      --strict                  exit with code 3 if the scan produced warnings
  -v, --verbose                 verbose logs
  -q, --quiet                   only report errors
  -h, --help                    show this help
  -V, --version                 show version

Exit codes:
  0 success, 1 fatal error, 2 invalid usage, 3 warnings with --strict

Started without arguments from a file manager (double click), the folder dialog
opens automatically.

Environment:
  NO_COLOR                      disable colored terminal output
`
