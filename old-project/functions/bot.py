from os import getenv, path, system
from dotenv import load_dotenv
import subprocess
from .helpers import *
from urllib.parse import quote
from requests import get

load_dotenv(override=True)

llama_completion_dir = getenv("LLAMA_COMPLETION_DIR")
llama_cpp_dir = getenv("LLAMA_CPP_DIR")

def run_command(command):

    if command:
        print(botPrint('The command I think you want to run is: ') + botPrint(command, 'White'))
        print(botPrint('Would you like to run this command? (y/N)', 'Yellow'))
        user_input = input()

        if user_input == "Y" or user_input == "y":
            print(botPrint('Running command: ') + botPrint(command, 'White'))
            system(command)
            exit()
        else:
            print(botPrint("Okay, I won't run the command."))
            exit()
    else:
        print (botPrint('An error ocurred. Please, try again!', 'Red'))

def generate_llama_prompt(prompt, option, Tokens = 100):
    llama_model = path.join(llama_cpp_dir) + path.join(getenv(option + "_LLAMA_MODEL"))
    gpu = getenv('GPU')
    gpu_layers = ''
    
    if (gpu == 'YES'):
        layers = getenv('GPU_LAYERS')
        gpu_layers = f"--n-gpu-layers = {(layers)} "

    prompt = (
        path.join(llama_cpp_dir)
        + f"main -m  {(llama_model)} -p '"
        + getenv(option + '_TEXT_START').replace(r'\n', '\n')
        + prompt
        + getenv(option + '_TEXT_END').replace(r'\n', '\n')
        + f"' -n {(getenv(option + '_TOKENS'))} -e "
        + f"--top-p {(getenv(option + '_TOP_P'))} "
        + f"--top-k {(getenv(option + '_TOP_K'))} "
        + f"--ctx-size {(getenv(option + '_CTX'))} "
        + f"--repeat-penalty {(getenv(option + '_R_PENALTY'))} "
        + gpu_layers
        + ' --log-disable'
    )
    return prompt


# Builder for instancing env variables and generating the prompt
def run_llama_builder(prompt, option, token = None):

    text_delimiter = str(getenv(option + "_TEXT_DELIMITER"))
    text_end = str(getenv(option + "_TEXT_END"))
    token = token if token is not None else getenv(option + '_TOKENS')

    # Generate the builder passing the variables and getting the envs
    builder = generate_llama_prompt(prompt, option, token)

    # Run the llama.cpp in a subprocess
    llamaOutput = subprocess.Popen(builder, shell=True,stdout=subprocess.PIPE, stdin=subprocess.DEVNULL)
    
    # Get the delimiters based on .env variables and stop the llama if anything matches
    delimiter_count = 0
    result = None
    llamaLog = ''
    i = 0
    
    while True:
        i += 1
        llamaOutputLine = str(llamaOutput.stdout.readline().decode('utf-8'))
        llamaLog += llamaOutputLine

        if text_delimiter in llamaOutputLine:
            delimiter_count += 1
            if delimiter_count > 1 and text_end == '': # Has a delimiter and dont have end text, probably a question
                result = llamaOutputLine.replace(text_delimiter, '').strip()
                llamaOutput.terminate()
            elif delimiter_count == 1 and text_end != '': # Has a delimiter and a end text, probably a command
                result = find_between(llamaOutputLine, text_end, text_delimiter) # Get value between the end text and delimiter to get the command only
                llamaOutput.terminate()
        elif i > 5:
            print(botPrint('Please, try again!', 'Red'))
            llamaOutput.terminate()
            break
        
        if result is not None:
            result
            if option == 'C':
                run_command(result)
            else:
                print(botPrint(result))
                break
        else:
            pass

def run_wiki_summary(param):
    searchParam = quote(param)
    wikiUrl = 'https://en.wikipedia.org/w/api.php?format=json&action=query&prop=extracts&exintro&explaintext&redirects=1&titles='+searchParam
    try:
        response = get(wikiUrl).json()
        data = response['query']['pages']
        first_key = next(iter(data))
        if first_key == '-1':
            print(botPrint('No result find!', 'Grey'))
        else:
            summary = data[first_key]['extract']
            print(f"\n {(botPrint(summary))} \n")
    except:
        print(botPrint('Request error, try again!', 'Red'))
    
