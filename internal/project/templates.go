package project

// Language scaffold templates, ported verbatim from dotfiles/scripts/project-init.sh.
// Single-field templates ({{.}}) take the project name; the java/go templates
// take a small struct. Static .gitignore/config files are plain constants.

// ── Python ──────────────────────────────────────────────────────────────────

const pyprojectTmpl = `[project]
name = "{{.}}"
version = "0.1.0"
requires-python = ">=3.12"
dependencies = []

[dependency-groups]
dev = [
    "pytest>=8.0",
    "ruff>=0.6",
]
`

const pyMainTmpl = `def main() -> None:
    print("Hello from {{.}}!")


if __name__ == "__main__":
    main()
`

const pyTestTmpl = `from {{.}}.main import main


def test_main(capsys):
    main()
    captured = capsys.readouterr()
    assert "Hello from {{.}}!" in captured.out
`

const pythonGitignore = `__pycache__/
*.pyc
.venv/
*.egg-info/
dist/
.pytest_cache/
.mypy_cache/
.ruff_cache/
`

// ── Java ──────────────────────────────────────────────────────────────────

const pomTmpl = `<?xml version="1.0" encoding="UTF-8"?>
<project xmlns="http://maven.apache.org/POM/4.0.0"
         xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance"
         xsi:schemaLocation="http://maven.apache.org/POM/4.0.0
         http://maven.apache.org/xsd/maven-4.0.0.xsd">
    <modelVersion>4.0.0</modelVersion>

    <groupId>{{.GroupID}}</groupId>
    <artifactId>{{.Name}}</artifactId>
    <version>1.0.0</version>
    <packaging>jar</packaging>

    <properties>
        <maven.compiler.source>17</maven.compiler.source>
        <maven.compiler.target>17</maven.compiler.target>
        <project.build.sourceEncoding>UTF-8</project.build.sourceEncoding>
    </properties>

    <dependencies>
        <dependency>
            <groupId>org.junit.jupiter</groupId>
            <artifactId>junit-jupiter</artifactId>
            <version>5.10.0</version>
            <scope>test</scope>
        </dependency>
    </dependencies>

    <build>
        <plugins>
            <plugin>
                <groupId>org.apache.maven.plugins</groupId>
                <artifactId>maven-surefire-plugin</artifactId>
                <version>3.2.0</version>
            </plugin>
        </plugins>
    </build>
</project>
`

const javaMainTmpl = `package {{.GroupID}};

public class {{.Class}} {
    public static void main(String[] args) {
        System.out.println("Hello from {{.Name}}!");
    }
}
`

const javaTestTmpl = `package {{.GroupID}};

import org.junit.jupiter.api.Test;
import static org.junit.jupiter.api.Assertions.*;

class {{.Class}}Test {
    @Test
    void mainRuns() {
        assertDoesNotThrow(() -> {{.Class}}.main(null));
    }
}
`

const javaGitignore = `target/
*.class
.idea/
*.iml
.classpath
.project
.settings/
`

// ── TypeScript ──────────────────────────────────────────────────────────────

const packageJSONTmpl = `{
  "name": "{{.}}",
  "version": "1.0.0",
  "type": "module",
  "scripts": {
    "build": "tsc",
    "start": "node dist/index.js",
    "test": "node --test dist/test/"
  },
  "devDependencies": {
    "typescript": "^5.6.0",
    "@types/node": "^22.0.0"
  }
}
`

const tsconfig = `{
  "compilerOptions": {
    "target": "ES2022",
    "module": "ES2022",
    "moduleResolution": "bundler",
    "outDir": "dist",
    "rootDir": ".",
    "strict": true,
    "esModuleInterop": true,
    "skipLibCheck": true,
    "forceConsistentCasingInFileNames": true
  },
  "include": ["src", "test"]
}
`

const tsIndexTmpl = `export function main(): void {
  console.log("Hello from {{.}}!");
}

main();
`

const tsTest = `import { describe, it } from "node:test";
import assert from "node:assert/strict";
import { main } from "../src/index.js";

describe("main", () => {
  it("should not throw", () => {
    assert.doesNotThrow(() => main());
  });
});
`

const tsGitignore = `node_modules/
dist/
*.js
*.d.ts
!jest.config.js
`

// ── Go ──────────────────────────────────────────────────────────────────

const goModTmpl = `module {{.Module}}

go 1.23
`

const goMainTmpl = `package main

import "fmt"

func main() {
	fmt.Println("Hello from {{.Module}}!")
}
`

const goGitignore = `/bin/
*.exe
*.test
*.out
dist/
`

// ── C/C++ ──────────────────────────────────────────────────────────────────

const cmakeTmpl = `cmake_minimum_required(VERSION 3.20)
project({{.}} CXX)

set(CMAKE_CXX_STANDARD 20)
set(CMAKE_CXX_STANDARD_REQUIRED ON)

add_executable({{.}} src/main.cpp)

enable_testing()
add_executable(test_main tests/test_main.cpp)
target_link_libraries(test_main PRIVATE {{.}})
add_test(NAME main_test COMMAND test_main)
`

const cppMainTmpl = `#include <iostream>

int main() {
    std::cout << "Hello from {{.}}!" << std::endl;
    return 0;
}
`

const cppTest = `#include <cassert>
#include <iostream>

int main() {
    // Add tests here
    assert(true);
    std::cout << "All tests passed!" << std::endl;
    return 0;
}
`

const cppGitignore = `build/
*.o
*.out
*.exe
compile_commands.json
.cache/
`

// ── Rust ──────────────────────────────────────────────────────────────────

const cargoTmpl = `[package]
name = "{{.}}"
version = "0.1.0"
edition = "2021"

[dependencies]
`

const rustMainTmpl = `fn main() {
    println!("Hello from {{.}}!");
}
`

const rustGitignore = `/target/
*.rs.bk
`
