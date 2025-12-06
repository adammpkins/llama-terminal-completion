# LlamaTerm

![LlamaTerm Logo](llama-md.png)

Ever wish you could look up Linux commands or ask questions and receive responses from the terminal? You probably need a paid service, an API key with paid usage, or at least an internet connection, right? Not with Llama Terminal Completion. Instead, we'll run a Large Language Model (think ChatGPT) locally, on your personal machine, and generate responses from there.

Website: [http://adammpkins.github.io/llamaterm](https://adammpkins.github.io/llamaterm)

## Table of Contents
- [LlamaTerm](#llamaterm)
  - [Table of Contents](#table-of-contents)
  - [Installation](#installation)
      - [Scripted installation](#scripted-installation)
      - [Manual installation](#manual-installation)
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
### Scripted installation
This installation method clones and compiles the llama.cpp repository using huggyllama/llama-7b as the default model.
1. Clone the 'llama.cpp' repository to your local machine:
```bash
git clone https://github.com/ggerganov/llama.cpp.git
```
2. Enter the repository folder:
```bash
cd llama-terminal-completion/
```
3. Run the script (the process may take a while):
```bash
./configure_llama_linux.sh
```
### Manual installation
#### Llama.cpp installation
1. Clone the 'llama.cpp' repository to your local machine:
```bash
git clone https://github.com/ggerganov/llama.cpp.git
```
2. Build the llama.cpp library by following the instructions in the llama.cpp repository. A good tutorial for this can be found on the official llama.cpp [README](https://github.com/ggerganov/llama.cpp/blob/master/README.md)

#### Llama Terminal Completion installation
1. Clone the llama-terminal-completion repository to your local machine:
```bash
git clone https://github.com/adammpkins/llama-terminal-completion.git
```
2. Create a `.env` file by copying from `.env_example`:
```bash
cp .env_example .env
```
3. Set up the environment variables (see below)


### Environment Variables

Before using this script, if installed manually, you need to set up the `LLAMA_COMPLETION_DIR`, `LLAMA_CPP_DIR`, `Q_LLAMA_MODEL` and `C_LLAMA_MODEL` environment variables. These variables point to the directories where the `llama-terminal-completion` and `llama.cpp` files are located, respectively. You can set these variables in your `.env` configuration file like this:

```bash
LLAMA_COMPLETION_DIR="/path/to/llama-terminal-completion/"
LLAMA_CPP_DIR="/path/to/llama.cpp/"
Q_LLAMA_MODEL=="name_of_model_file.gguf"
C_LLAMA_MODEL=="name_of_model_file.gguf"
```

If your models are organized into 7B, 13B, and 30B and 65B folders, you can set the `LLAMATERM_MODEL_FILE` variable to the name of the model file you want to use by prepending the model file name with the folder name. For example, if you want to use the 12B model, you would set the `LLAMATERM_MODEL_FILE` variable to `12B/name_of_model_file.gguf`.

Replace /path/to/llama-terminal-completion/ and /path/to/llama.cpp/ with the actual paths to the respective directories on your system.

You can also change the question and command prompt to a text to your liking, as well as the tokens and temperature. Everything can be done by changing the variables in the .env file. Variables starting with `C_` are for commands, and variables starting with `Q_` are for questions.

## Usage
Open a terminal window.

Navigate to the directory where the ask_llama.py script is located.

Run the script with the desired options. Here are some examples:

- To generate a Linux command based on a prompt:
    ```bash
    python3 ask_llama.py -c "list the contents of the current directory"
    ```
- To ask a question to the virtual assistant:

    ```bash
    python3 ask_llama.py -q "How does photosynthesis work?"
    ```
- To search for a wiki summary with the virtual assistant:

    ```bash
    python3 ask_llama.py -w "PHP"
    ```

For more options, you can run:

```bash
python3 ask_llama.py --help
```
Its output is as follows:
    
```bash
usage: ask_llama.py [-h] [-w Wiki] [-c Command] [-q Question] [-n Token]

options:
  -h, --help   show this help message and exit
  -w Wiki      Get a wiki summary by title
  -c Command   Predict a command by text
  -q Question  Ask a question to the virtual assistant
  -n Token     (Optional) Number of tokens to predict
```

### Alias
You can create an alias for the script in your shell configuration file (e.g., `.bashrc` or `.zshrc`) like this:

```bash
alias ask="python3 /path/to/llama-terminal-completion/ask_llama.py -c"
```

Then you can run the script like this:

```bash
ask "list the contents of the current directory"
```

## Contributing
Contributions to this project are welcome! Feel free to fork the repository, make changes, and submit pull requests.

## License
This project is licensed under the [MIT License](https://choosealicense.com/licenses/mit/)


