# LlamaTerm

![LlamaTerm Logo](llama-md.png)

Ever wish you could look up Linux commands or ask questions and receive responses from the terminal? You probably need a paid service, an API key with paid usage, or at least an internet connection, right? Not with Llama Terminal Completion. Instead, we'll run a Large Language Model (think ChatGPT) locally, on your personal machine, and generate responses from there.

Website: [http://adammpkins.github.io/llamaterm](https://adammpkins.github.io/llamaterm)

## Table of Contents
- [LlamaTerm](#llamaterm)
  - [Table of Contents](#table-of-contents)
  - [Installation](#installation)
    - [Llama.cpp installation](#llamacpp-installation)
    - [Llama Terminal Completion installation](#llama-terminal-completion-installation)
    - [Environment Variables](#environment-variables)
  - [Usage](#usage)
    - [Alias](#alias)
  - [Contributing](#contributing)
  - [License](#license)
    
![image](https://github.com/adammpkins/llama-terminal-completion/blob/main/readme.gif)

This Python script interacts with the [llama.cpp](https://github.com/ggerganov/llama.cpp) library to provide virtual assistant capabilities through the command line. It allows you to ask questions and receive intelligent responses, as well as generate Linux commands based on your prompts.



## Installation

### Llama.cpp installation
1. Clone the 'llama.cpp' repository to your local machine
```bash
git clone https://github.com/ggerganov/llama.cpp.git
```
2. Build the llama.cpp library by following the instructions in the llama.cpp repository. A good tutorial for this can be found on the official llama.cpp [README](https://github.com/ggerganov/llama.cpp/blob/master/README.md)

### Llama Terminal Completion installation
1. Clone the llama-terminal-completion repository to your local machine:
```bash
git clone https://github.com/adammpkins/llama-terminal-completion.git
```
2. Set up the environment variables (see below)


### Environment Variables

Before using this script, you need to set up the `LLAMA_COMPLETION_DIR`, `LLAMA_CPP_DIR`, and `LLAMATERM_MODEL_FILE` environment variables. These variables point to the directories where the `llama-terminal-completion` and `llama.cpp` files are located, respectively. You can set these variables in your shell configuration file (e.g., `.bashrc` or `.zshrc`) like this:

```bash
export LLAMA_COMPLETION_DIR="/path/to/llama-terminal-completion/"
export LLAMA_CPP_DIR="/path/to/llama.cpp/"
export LLAMATERM_MODEL_FILE="name_of_model_file.gguf"
```

If your models are organized into 7B, 13B, and 30B and 65B folders, you can set the `LLAMATERM_MODEL_FILE` variable to the name of the model file you want to use by prepending the model file name with the folder name. For example, if you want to use the 12B model, you would set the `LLAMATERM_MODEL_FILE` variable to `12B/name_of_model_file.gguf`.

Replace /path/to/llama-terminal-completion/ and /path/to/llama.cpp/ with the actual paths to the respective directories on your system.

## Usage
Open a terminal window.

Navigate to the directory where the ask_llama.py script is located.

Run the script with the desired options. Here are some examples:

- To generate a Linux command based on a prompt:
    ```bash
    python3 ask_llama.py "list the contents of the current directory"
    ```
- To ask a question to the virtual assistant:

    ```bash
    python3 ask_llama.py -q "How does photosynthesis work?"
    ```
- To clear the history of commands:
    
    ```bash
    python3 ask_llama.py -ch
    ```

For more options, you can run:

```bash
python3 ask_llama.py --help
```
Its output is as follows:
    
```bash
Usage: python3 ask_llama.py [prompt]
Example: python3 ask_llama.py 'list all files in the current directory'
Options:
-q                ask a question to the virtual assistant
-ch               clear the history of commands
-cqh              clear the history of questions
-h                show the history of commands
-qh               show the history of questions
-v                show the version of llama-terminal-completion
--help            show this help message and exit
```

### Alias
You can create an alias for the script in your shell configuration file (e.g., `.bashrc` or `.zshrc`) like this:

```bash
alias ask="python3 /path/to/llama-terminal-completion/ask_llama.py"
```

Then you can run the script like this:

```bash
ask "list the contents of the current directory"
```

## Contributing
Contributions to this project are welcome! Feel free to fork the repository, make changes, and submit pull requests.

## License
This project is licensed under the [MIT License](https://choosealicense.com/licenses/mit/)


