from functions import *


def main():
    # Get the arguments passed to the program
    args = get_args()

    # Set the prompt to the first argument if it exists
    if len(args) > 0:
        prompt = args[0]

    # If no arguments are passed, print help
    if len(args) == 0:
        print_help()
        exit()

    # If the argument is qh, print the question history
    if "-qh" in args:
        print_question_history()
        exit()

    # If the argument is ch, clear the command history
    if "-ch" in args:
        clear_history()
        print("\033[92m" + "History file cleared" + "\033[0m")
        exit()

    # If the argument is cqh, clear the question history
    if "-cqh" in args:
        clear_question_history()
        print("\033[92m" + "Question history file cleared" + "\033[0m")
        exit()

    # If the argument is h, print the command history
    if "-h" in args:
        print_history()
        exit()

    # If the argument is --help, print the help
    if "--help" in args:
        print_help()
        exit()

    # If the argument is -v, print the version
    if "-v" in args:
        print_version()
        exit()

    # If the argument is -q, ask a question
    if "-q" in args:
        # Set the prompt to the second argument,since the first is -q
        prompt = args[1]
        clear_question_file()
        run_llama_question(prompt)
        response = process_llama_question()
        print("\033[92m" + response + "\033[0m")
        exit()

    clear_output_file()
    run_llama_request(prompt)
    response = process_llama_output()
    prompt_user(response)


if __name__ == "__main__":
    main()
