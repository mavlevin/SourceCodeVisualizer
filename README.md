
# Source Code Visualizer

Visualize the code distribution in a project.

![Visualizing the source code distribution in Apache httpd](visualizecodevideo.gif)

## Applications

Applications include:

* Visualizing code distribution for more educated development and managment plans
* Helping identify needlessly complex code modules
* Mapping the attack surface from a vulnerability analysis perspective

## Current Features

- [X] Visualize how a projects source code is distributed among files and folders
- [X] Zoom, hover and drag to explore the source code map
- [X] Set a custom file extension filter to only view relevant source code files

## Building Source Code Visualizer

Simply ```go build visualize.go``` to build Source Code Visualizer.

Execute ```visualize.exe``` to run Source Code Visualizer, which will automatically open the default browser displaying the program's web UI.

## Road Map

- [ ] Improve the treemap display to better show folders
- [ ] Implement CSRF Tokens to mitigate possible information disclosure
- [ ] Implement autocomplete in the directory input field (according to available files on computer)
- [ ] Allow custom file extension filters
