#!/bin/bash
LLAMA_REPO='https://github.com/ggerganov/llama.cpp';
LLAMA_MODEL='llama-7b';
LLAMA_MODEL_FOLDER='models/7B';
LLAMA_TERMIAL=$(dirname $0);

function setEnv() {
  sed -i~ "\|^$1=|s|=\(.*\)|=$2|" .env;
}

echo -n "This script download and configue the default llama-7b model inside the bot folder";
read -p "This will erase the existing .env and any llama.cpp folder inside this repo, are you sure (Yy/Nn?) " -n 1 -r;
echo;
if [[ $REPLY =~ ^[Yy]$ ]]
then
    cd $LLAMA_TERMIAL;
    COMPLETION_DIR=$(pwd);
    yes | cp -f .env_example .env;
    setEnv 'LLAMA_COMPLETION_DIR' $COMPLETION_DIR/;
    setEnv 'LLAMA_CPP_DIR' "${COMPLETION_DIR}/llama.cpp/";
    setEnv 'LLAMA_MODEL' "/${LLAMA_MODEL_FOLDER}/ggml-model-q4_0.gguf";
    
    rm -rf 'llama.cpp';
    git clone $LLAMA_REPO;
    cd 'llama.cpp';
    pip install -r requirements.txt
    make;
    cd "${COMPLETION_DIR}/llama.cpp";
    mkdir -p $LLAMA_MODEL_FOLDER;
    cd "${COMPLETION_DIR}/llama.cpp/$LLAMA_MODEL_FOLDER"
    wget "https://huggingface.co/huggyllama/${LLAMA_MODEL}/resolve/main/pytorch_model-00001-of-00002.bin";
    wget "https://huggingface.co/huggyllama/${LLAMA_MODEL}/resolve/main/pytorch_model-00002-of-00002.bin";
    wget "https://huggingface.co/huggyllama/${LLAMA_MODEL}/resolve/main/tokenizer.model";
    wget "https://huggingface.co/huggyllama/${LLAMA_MODEL}/resolve/main/config.json";
    cd "${COMPLETION_DIR}/llama.cpp";
    python3 convert.py ./$LLAMA_MODEL_FOLDER --ctx 4096;
    ./quantize ./${LLAMA_MODEL_FOLDER}/ggml-model-f16.gguf ./${LLAMA_MODEL_FOLDER}/ggml-model-q4_0.gguf q4_0;
    echo "Script $0 finished";
fi


