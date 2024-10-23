const path = require('path')

const dirName = () => {
    const correct = []

    let splitDir = ""
    if (process.platform === "win32") {
        splitDir = __dirname.split('\\')
    } else {
        splitDir = __dirname.split('/')
    }

    for (let i = 0; i < splitDir.length - 1; i++) {
        correct.push(splitDir[i])
    }

    if (process.platform === "win32") {
        return correct.join("\\")
    } else {
        return correct.join("/")
    }
}

module.exports = {
    entry: ['@babel/polyfill'],
    output: {
        path: dirName() + "/dist"
    },
    module: {
        rules: [
            {
                test: /\.css$/i,
                exclude: /node_modules/,
                use: [
                    'style-loader',
                    {
                        loader: 'css-loader',
                        options: {
                            modules: true,
                        },
                    },
                ],
            },
            {
                test: /\.(js|jsx)$/,
                exclude: /node_modules/,
                use: {
                    loader: "babel-loader",
                    options: {
                        "presets": [
                            [
                                "@babel/preset-env",
                                {
                                    "useBuiltIns": "entry"
                                }
                            ],
                            "@babel/preset-react"
                        ],
                        "plugins": [
                            "@babel/plugin-proposal-class-properties",
                            "@babel/plugin-proposal-export-default-from",
                            "react-hot-loader/babel"
                        ]
                    }
                }
            },
            {
                test: /\.html$/,
                use: [
                    {
                        loader: "html-loader",
                    }
                ]
            }
        ]
    },
    resolve: {
        fallback: {
            "react/jsx-runtime": "react/jsx-runtime.js",
            "react/jsx-dev-runtime": "react/jsx-dev-runtime.js",
        },
        extensions: ['.js', '.jsx', '.mjs', '.json'],
        modules: ['node_modules', path.resolve(__dirname, './')],
        resolve: {
            alias: {
              'react/jsx-runtime': 'react/jsx-runtime.js',
            },
        }
    },
};