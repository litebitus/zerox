#compdef zerox
compdef _zerox zerox
zstyle ':completion:*:*:*zerox:*:values' list-grouped false
zstyle ':completion:*:*:*zerox:*' menu select

# zsh completion for zerox, listing only what comes next of the binary's own candidates.

_zerox()
{
    local -i directive
    local out line value description typed flag held warning
    local -a lines candidates values prefix_option options order
    local -a flag_names flag_descriptions described_values

    out=$(ZEROX_COMPLETE_AFTER_CURSOR="${(pj:\n:)${(@Q)words[CURRENT+1,-1]}}" \
        ${~${(Q)words[1]}} {{.Request}} "${(@Q)words[2,CURRENT-1]}" "${(Q)PREFIX}" 2>/dev/null) || return 1
    lines=("${(@f)out}")
    [[ ${lines[-1]} == :<-> ]] || return 1
    directive=${lines[-1]#:}
    (( directive & 1 )) && return 1

    for line in "${(@)lines[1,-2]}"; do
        if [[ $line == "_activeHelp_ "* ]]; then
            warning=${line#_activeHelp_ }
            compadd -x "%F{11}${warning//\%/%%}%f"
        elif [[ -n $line ]]; then
            candidates+=("$line")
        fi
    done

    typed=$PREFIX
    if [[ $typed == -*=* ]]; then
        flag=${typed%%=*}=
        typed=${typed#*=}
        prefix_option=(-P "$flag")
    fi
    held=${(M)typed##*[/?&=]}
    for line in "${candidates[@]}"; do
        values+=("${line%%$'\t'*}")
        [[ ${values[-1][1,$#held]} == "$held" ]] || held=""
    done

    if (( directive & 8 )); then
        _files "${prefix_option[@]}" -g "*.(${(j:|:)values})"
        return
    fi
    if (( directive & 16 )); then
        _files "${prefix_option[@]}" -/ ${values[1]:+-W} ${values[1]}
        return
    fi

    for line in "${candidates[@]}"; do
        value=${line%%$'\t'*}
        description=""
        [[ $line == *$'\t'* ]] && description=${line#*$'\t'}
        value=${value[$#held+1,-1]}
        if [[ $value == -* ]]; then
            flag_names+=("$value")
            flag_descriptions+=("$description")
        else
            described_values+=("${value//:/\\:}${description:+:$description}")
        fi
    done
    options=(-Q -M 'l:|=* l:[:/]|=* r:|=*')
    (( directive & 2 )) && options+=(-S '')
    [[ -n $flag$held ]] && compset -P "${(b)flag}${(b)held}"
    (( directive & 32 )) && order=(-V)
    if (( ${#flag_names} + ${#described_values} )); then
        comppostfuncs+=(__zerox_type_no_shared_run)
        (( ${#candidates} == 1 )) && [[ ${candidates[1]} == *$'\t'?* ]] &&
            comppostfuncs+=(__zerox_list_a_lone_description)
        local -i described=1
        (( ${#flag_names} )) && __zerox_add_flags && described=0
        (( ${#described_values} )) &&
            _describe "${order[@]}" -t values "values" described_values "${options[@]}" && described=0
        (( described == 0 )) && return 0
    fi
    (( directive & 4 )) && return 1
    _files "${prefix_option[@]}"
}

__zerox_add_flags()
{
    local -a shown lefts shown_descriptions displays hidden group=(-J options)
    local -A paired
    local left said
    local -i long short width
    (( ${#order} )) && group=(-V options)
    for (( long = 1; long <= ${#flag_names}; long++ )); do
        [[ ${flag_names[long]} == --* ]] || continue
        left=${flag_names[long]}
        said=${flag_descriptions[long]}
        for (( short = 1; short <= ${#flag_names}; short++ )); do
            [[ ${flag_names[short]} == -[^-] && -z ${paired[$short]} ]] || continue
            [[ -n $said && ${flag_descriptions[short]} == "$said" ]] || continue
            left+=" | ${flag_names[short]}"
            paired[$short]=1
            hidden+=("${flag_names[short]}")
            break
        done
        shown+=("${flag_names[long]}")
        lefts+=("$left")
        shown_descriptions+=("${flag_descriptions[long]}")
    done
    for (( short = 1; short <= ${#flag_names}; short++ )); do
        [[ ${flag_names[short]} == --* || -n ${paired[$short]} ]] && continue
        shown+=("${flag_names[short]}")
        lefts+=("${flag_names[short]}")
        shown_descriptions+=("${flag_descriptions[short]}")
    done
    for left in "${lefts[@]}"; do
        (( ${#left} > width )) && width=${#left}
    done
    for (( long = 1; long <= ${#shown}; long++ )); do
        displays+=("${(r:width:)lefts[long]}${shown_descriptions[long]:+  -- ${shown_descriptions[long]}}")
    done
    compadd "${group[@]}" -l -d displays "${options[@]}" -a shown || return
    (( ${#hidden} )) && compadd "${group[@]}" -n "${options[@]}" -a hidden
    return 0
}

__zerox_type_no_shared_run()
{
    [[ ${compstate[insert]} == *unambiguous* ]] && (( compstate[nmatches] > 1 )) || return
    __zerox_list_then_menu
}

__zerox_list_a_lone_description()
{
    (( compstate[nmatches] == 1 )) || return
    compstate[list]='list force'
}

__zerox_list_then_menu()
{
    if [[ ${compstate[insert]} == automenu* && -n ${compstate[old_list]} &&
        $LBUFFER == "$__zerox_listed_line" ]]; then
        compstate[insert]=menu
    else
        compstate[insert]=''
        typeset -g __zerox_listed_line=$LBUFFER
    fi
}

if [ "$funcstack[1]" = "_zerox" ]; then
    _zerox
fi
