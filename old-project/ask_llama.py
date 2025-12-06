from functions.bot import *
from functions.cli import *
from os import sys

def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('-w', metavar='Wiki', type=str, help='Get a wiki summary by title')
    parser.add_argument('-c', metavar='Command', type=str, help='Predict a command by text')
    parser.add_argument('-q', metavar='Question', type=str, help='Ask a question to the virtual assistant')
    parser.add_argument('-n', metavar='Token', type=int, help='(Optional) Number of tokens to predict')
    args = parser.parse_args()

    try:
        args = parser.parse_args()
        if len(sys.argv) == 1:
            showLogo()
            parser.print_help()
    except:
        showLogo()
        sys.exit(0)

    if args.c:
        run_llama_builder(args.c, 'C', args.n)
        sys.exit(0)
    elif args.q:
        run_llama_builder(args.q, 'Q', args.n)
        sys.exit(0)
    elif args.w:
        run_wiki_summary(args.w)
        sys.exit(0)


if __name__ == "__main__":
    main()
