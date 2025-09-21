"""
A generated module for Dagger Pipeline functions

This module has been generated via dagger init and serves as a reference to
basic module structure as you get started with Dagger.

Two functions have been pre-created:
- number_of_files: returns the number of files in a directory
- hello: returns a string with a welcome message

You can learn more about Dagger functions at https://docs.dagger.io/functions
"""

import dagger
from dagger import dag, function, object_type


@object_type
class DaggerPipeline:
    @function
    def number_of_files(self, directory: dagger.Directory) -> int:
        """
        Returns the number of files in a directory
        """
        return len(directory.entries())

    @function
    def hello(self, name: str = "world") -> str:
        """
        Returns a string with a welcome message
        """
        return f"Hello, {name}!"
