from sys import argv
from .helpers import botPrint
import argparse
from os import getenv, get_terminal_size

def showArt():
  terminal_size = get_terminal_size()
  # Can create a ASCII art here
  print(botPrint("""
  
  LlamaTerm
  """, 'White'))

def showLogo():

  showArt()
  fileName = argv[0]
  print("\n" + botPrint('V1.0.0', 'Grey'))
  print(botPrint('• Wiki Summary: ') + botPrint(f'python {(fileName)} -w "PHP"', 'Blue'))
  print(botPrint('• Question: ') + botPrint(f'python {(fileName)} -q "How does photosynthesis work?"', 'Blue'))
  print(botPrint('• Command: ') + botPrint(f'python {(fileName)} -c "List the contents of the current directory"', 'Blue'))
  print()