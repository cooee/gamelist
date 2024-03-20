
import axios from "axios";
const clientID = "fIJXLAYLBozRxXpC"
export async function access_token(code) {
    const http = axios.create({
        baseURL: "",
        headers: {
            'Content-Type': 'code'
        },
    })
    // const url =
    //   "http://localhost:4980/get-access-token";
    const url =
        "https://lm.2048.net:4980/get-access-token";
    return http
        .post(url, {
            'code': code,
            'clientID': clientID
        },)
}

export async function getAuthCode() {
    const http = axios.create({
        baseURL: "",
        headers: {
            'Content-Type': 'code'
        }
    })
    // 添加请求拦截器
    http.interceptors.request.use(function (config) {
        // 在发送请求之前做些什么
        // 随便写个值 绕过if判段
        if (config.method == 'get') {
            config.data = true
        }
        config.headers['Content-Type'] = 'code'
        config.headers['Origin'] = '*'
        return config;
    }, function (error) {
        // 对请求错误做些什么
        return Promise.reject(error);
    });

    const u = navigator.userAgent;

    let isAndroid = u.indexOf('Android') > -1 || u.indexOf('Adr') > -1;   //判断是否是 android终端
    let isIOS = !!u.match(/\(i[^;]+;( U;)? CPU.+Mac OS X/);     //判断是否是 iOS终端


    let agent = isAndroid == true ? "gateio/android" : "gateio/ios";

    console.log(u, isAndroid, isIOS, agent);

    const url =
        `https://lm.2048.net:4980/auth?name=https://lm.2048.net/test/index.html&agent=${agent}&clientID=${clientID}`;
    return http
        .get(url, {
            headers: { "Content-Type": "code", "User-Agent": agent, "Origin": "*" },
        })
}


export function getQuerys(e) {
    if (!e) return "";
    var t = {},
        r = [],
        n = "",
        a = "";
    try {
        var i = [];
        if (e.indexOf("?") >= 0 && (i = e.substring(e.indexOf("?") + 1, e.length).split("&")), i.length > 0) for (var o in i) n = (r = i[o].split("="))[0],
            a = r[1],
            t[n] = a
    } catch (s) {
        t = {}
    }
    return t
}

export function user_profile(token) {
    const url = "https://lm.2048.net:4980/user_profile";
    return axios.request({
        headers: {
            Authorization: `Bearer ${token}`
        },
        method: "GET",
        url: url
    })
}