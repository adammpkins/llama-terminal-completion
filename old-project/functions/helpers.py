def find_between( s, first, last ):
    try:
        start = s.index( first ) + len( first )
        end = s.index( last, start )
        return s[start:end]
    except ValueError:
        return ""

def botPrint(value, color_schema = 'Green'):

    normal_color = "\033[0m"
    colors = { 
        'Red': "\033[91m",
        'Green': "\033[92m",
        'Blue': "\033[94m",
        'Cyan': "\033[96m",
        'White': "\033[97m",
        'Yellow': "\033[93m",
        'Magenta': "\033[95m",
        'Grey': "\033[90m",
        'Black': "\033[90m",
        'Default': "\033[99m",
    }
    return (colors[color_schema] + value +  normal_color) 
