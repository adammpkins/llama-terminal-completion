import os
import sys
import time
from pprint import pprint

# Get the environment variables
# Example in .bashrc or .zshrc. Update the paths to match your system:
# export LLAMA_COMPLETION_DIR="$HOME/Development/llama-terminal-completion/"
# export LLAMA_CPP_DIR="$HOME/Development/llama-cpp/"
llama_completion_dir = os.environ["LLAMA_COMPLETION_DIR"]
llama_cpp_dir = os.environ["LLAMA_CPP_DIR"]


# Get the arguments passed to the program
def get_args():
    args = sys.argv

    # Remove the first argument which is the name of the program
    args.pop(0)
    return args


# Print the command history
def print_history():
    with open(llama_completion_dir + "history.txt", "r") as f:
        for line in f:
            print(line)


# Print the question history
def print_question_history():
    with open(
        llama_completion_dir + "question_history.txt",
        "r",
    ) as f:
        for line in f:
            print(line)


# Clear the command history file
def clear_history():
    with open(llama_completion_dir + "history.txt", "w") as f:
        f.write("")


# Clear the question history file
def clear_question_history():
    with open(
        llama_completion_dir + "question_history.txt",
        "w",
    ) as f:
        f.write("")


# Clear the command output file
def clear_output_file():
    with open(llama_completion_dir + "llama_output.txt", "w") as f:
        f.write("")


# Clear the question output file
def clear_question_file():
    with open(llama_completion_dir + "llama_question.txt", "w") as f:
        f.write("")


# Print the help message
def print_help():
    print("\033[92m" + "Usage: python3 ask_llama.py [prompt]" + "\033[0m")
    print(
        "\033[92m"
        + "Example: python3 ask_llama.py 'list all files in the current directory'"
        + "\033[0m"
    )
    print("Options:")
    print("-q                ask a question to the virtual assistant")
    print("-ch               clear the history of commands")
    print("-cqh              clear the history of questions")
    print("-h                show the history of commands")
    print("-qh               show the history of questions")
    print("-v                show the version of llama-terminal-completion")
    print("--help            show this help message and exit")


# Print the version of llama-terminal-completion
def print_version():
    print("\033[92m" + "llama-terminal-completion v1.0.0" + "\033[0m")


# Run the llama command request
def run_llama_request(prompt):
    os.system(
        llama_cpp_dir
        + "main -m "
        + llama_cpp_dir
        + "/models/7B/ggml-model-q4_0.bin -p 'The following command is a single Linux command that will "
        + prompt
        + ".: $ `' -n 25 --top-p 0.5 --top-k 30 --ctx-size 256  --repeat-penalty 1.0 >> "
        + llama_completion_dir
        + "llama_output.txt 2>/dev/null"
    )


# Run the llama question request
def run_llama_question(prompt):
    os.system(
        llama_cpp_dir
        + "main -m "
        + llama_cpp_dir
        + "models/7B/ggml-model-q4_0.bin -p 'The following is a trancript of a conversation with a virtual assistant. The assistant only provides correct answers to questions. \n Assistant: What can I help you with today? \n User:"
        + prompt
        + "\n Assistant:' -n 100 --top-p 0.5 --top-k 30 --ctx-size 256  --repeat-penalty 1.0 >> "
        + llama_completion_dir
        + "llama_question.txt 2>/dev/null"
    )


# Process the output of llama question request
def process_llama_question():
    # Read the output file line by line
    with open(llama_completion_dir + "llama_question.txt", "r") as f:
        i = 0
        for line in f:
            i = i + 1
            if i == 4:
                response = line

                # response is the first occurence of "Assistant:" and is ended by a newline. So we split on the newline and after "Assistant"
                response = response.split("Assistant:")[1]
                response = response.split("\n")[0]

                # Log the question to the history file
                with open(
                    llama_completion_dir + "question_history.txt",
                    "a",
                ) as f:
                    f.write(
                        time.strftime("%Y-%m-%d %H:%M:%S", time.localtime())
                        + " - "
                        + response
                        + "\n"
                    )

        return response
    return None


# Process the output of llama command request
def process_llama_output():
    with open(llama_completion_dir + "llama_output.txt", "r") as f:
        for line in f:
            if line.__contains__("$"):
                response = line

                # The command is the text between the first and last backticks
                response = response.split("`")[1]
                response = response.split("`")[-1]
                command = response

                # Log the command to the history file
                with open(
                    llama_completion_dir + "history.txt",
                    "a",
                ) as f:
                    f.write(
                        time.strftime("%Y-%m-%d %H:%M:%S", time.localtime())
                        + " - "
                        + command
                        + "\n"
                    )

                return response
    return None


# Prompt the user to run the command
def prompt_user(response):
    # If the response is not None, then we have a command to run, so display it to the user
    if response:
        print(
            "\033[92m"
            + "The command I think you want to run is: "
            + response
            + "\033[0m"
        )

        # Ask the user if they want to run the command
        print("\033[92m" + "Would you like to run this command? (Y/n)" + "\033[0m")

        # Get the user input and run the command if the user wants to, otherwise exit
        user_input = input()

        # If the user presses enter, then we assume they want to run the command, otherwise we check if they entered Y, y, or n and respond accordingly
        if user_input == "Y" or user_input == "y" or user_input == "":
            print("\033[92m" + "Running command..." + "\033[0m")
            os.system(response)
            exit()
        else:
            print("\033[92m" + "Okay, I won't run the command." + "\033[0m")
            exit()
