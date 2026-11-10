/*
 * Go-gui
 * Author  : Mr.Shu
 * Website : www.cq6k.com
 *
 * Android 输入法桥的 JNI 实现。
 *
 * 为什么放 C：JNI 调用（FindClass / GetMethodID / CallVoidMethod）在 cgo 里
 * 需要绕方法表，写起来又长又容易错；这里把全部 JNI 逻辑收在 C 里，
 * Go 侧只调用几个简单函数（init / show / hide / sync），回调也只有一个。
 *
 * 线程：本文件里的 JNI 调用可从任意线程发出（Java 侧自己 runOnUiThread）；
 * 每次调用前确保线程已 Attach 到 JVM。
 */

#include <jni.h>
#include <android/native_activity.h>
#include <android/log.h>
#include <stdio.h>
#include <stdlib.h>

/* Go 侧导出（internal/android/ime.go 的 //export ggGoImeText） */
extern void ggGoImeText(char *text);

static ANativeActivity *g_act = NULL;
static JavaVM *g_vm = NULL;
static jobject g_bridge = NULL;
static jclass g_cls = NULL;
/* g_activity Activity 的全局引用。
 *
 * 不能直接用 ANativeActivity.clazz：那是个 local reference，
 * 在 onCreate 回调结束后随时可能失效（实测拿到 null ClassLoader）。
 * 在 onCreate 里用 NewGlobalRef 固定下来。 */
static jobject g_activity = NULL;

static void ime_log(const char *msg) {
    __android_log_print(ANDROID_LOG_ERROR, "gogui", "ime: %s", msg);
}

/* env_get 取当前线程的 JNIEnv（没有就 Attach）。 */
static JNIEnv *env_get(void) {
    JNIEnv *env = NULL;
    if (g_vm == NULL) {
        return NULL;
    }
    if ((*g_vm)->GetEnv(g_vm, (void **) &env, JNI_VERSION_1_6) != JNI_OK) {
        if ((*g_vm)->AttachCurrentThread(g_vm, &env, NULL) != JNI_OK) {
            return NULL;
        }
    }
    return env;
}

/* 把 Java 侧挂起的异常打到 logcat（只用于诊断，打完清掉）。 */
static void log_exception(JNIEnv *env) {
    if ((*env)->ExceptionCheck(env)) {
        (*env)->ExceptionDescribe(env);
        (*env)->ExceptionClear(env);
    }
}

/* gg_ime_set_activity 由 Go 侧在拿到 ANativeActivity 后调用。 */
void gg_ime_set_activity(ANativeActivity *a) {
    g_act = a;
    if (a != NULL) {
        g_vm = a->vm;
    }
}

/* gg_ime_capture 在 ANativeActivity_onCreate 回调里固定 Activity 全局引用。
 *
 * 必须在这里做：回调参数里的 env / clazz 只在本次回调有效。 */
void gg_ime_capture(JNIEnv *env, jobject activity, JavaVM *vm) {
    if (env == NULL || activity == NULL) {
        return;
    }
    g_vm = vm;
    if (g_activity == NULL) {
        g_activity = (*env)->NewGlobalRef(env, activity);
    }
}

/* class_loader 取 Activity 的 ClassLoader。
 *
 * 不能直接用 FindClass：attached 出来的 native 线程没有 Java 侧的
 * context classloader，FindClass 只能看到系统类，应用 dex 里的类会找不到。 */
static jobject class_loader(JNIEnv *env) {
    jobject activity = g_activity;
    jclass act_cls;
    jmethodID m;
    jobject cl;

    if (activity == NULL) {
        ime_log("activity object is null");
        return NULL;
    }
    /* 优先 Context.getClassLoader()；不行再试 Application.getClassLoader() */
    act_cls = (*env)->GetObjectClass(env, activity);
    if (act_cls == NULL) {
        ime_log("GetObjectClass(activity) failed");
        log_exception(env);
        return NULL;
    }
    m = (*env)->GetMethodID(env, act_cls, "getClassLoader", "()Ljava/lang/ClassLoader;");
    if (m != NULL) {
        cl = (*env)->CallObjectMethod(env, act_cls, m);
        if (cl != NULL) {
            return cl;
        }
        log_exception(env);
    } else {
        log_exception(env);
    }

    m = (*env)->GetMethodID(env, act_cls, "getApplication", "()Landroid/app/Application;");
    if (m == NULL) {
        ime_log("GetMethodID(getApplication) failed");
        log_exception(env);
        return NULL;
    }
    cl = (*env)->CallObjectMethod(env, act_cls, m);
    if (cl == NULL) {
        ime_log("getApplication() returned null");
        log_exception(env);
        return NULL;
    }
    {
        jclass app_cls = (*env)->GetObjectClass(env, cl);
        jmethodID get_cl = (*env)->GetMethodID(env, app_cls, "getClassLoader",
                                               "()Ljava/lang/ClassLoader;");
        if (get_cl == NULL) {
            ime_log("Application.getClassLoader method missing");
            log_exception(env);
            return NULL;
        }
        return (*env)->CallObjectMethod(env, cl, get_cl);
    }
}

/* load_class 用 ClassLoader 加载应用类（name 用点号分隔）。 */
static jclass load_class(JNIEnv *env, const char *name) {
    jobject cl = class_loader(env);
    jclass cl_cls;
    jmethodID load;
    jstring jname;
    jobject k;

    if (cl == NULL) {
        ime_log("no classloader");
        return NULL;
    }
    cl_cls = (*env)->GetObjectClass(env, cl);
    if (cl_cls == NULL) {
        ime_log("GetObjectClass(classloader) failed");
        log_exception(env);
        return NULL;
    }
    load = (*env)->GetMethodID(env, cl_cls, "loadClass",
                               "(Ljava/lang/String;)Ljava/lang/Class;");
    if (load == NULL) {
        ime_log("GetMethodID(loadClass) failed");
        log_exception(env);
        return NULL;
    }
    jname = (*env)->NewStringUTF(env, name);
    k = (*env)->CallObjectMethod(env, cl, load, jname);
    (*env)->DeleteLocalRef(env, jname);
    if ((*env)->ExceptionCheck(env)) {
        log_exception(env);
        return NULL;
    }
    if (k == NULL) {
        ime_log("loadClass returned null");
        return NULL;
    }
    return (jclass) k;
}

/* gg_ime_ready 桥是否已就绪（Java 侧交过对象或 init 成功）。 */
int gg_ime_ready(void) {
    return g_bridge != NULL ? 1 : 0;
}

/* gg_ime_init 创建 ImeBridge 实例（cls_name 用点号分隔，例如 com.gogui.gallery.ImeBridge）。 */
int gg_ime_init(const char *cls_name) {
    JNIEnv *env;
    jmethodID ctor;
    jobject obj;

    /* 正常路径：Java 侧（GalleryActivity）已经把桥交过来了 */
    if (g_bridge != NULL) {
        return 1;
    }
    if (g_act == NULL || g_activity == NULL) {
        ime_log("init: activity not ready");
        return 0;
    }
    env = env_get();
    if (env == NULL) {
        ime_log("init: no JNIEnv");
        return 0;
    }
    if (g_bridge != NULL) {
        return 1;
    }
    if (g_cls == NULL) {
        jclass k = load_class(env, cls_name);
        if (k == NULL) {
            ime_log("init: class not found");
            return 0;
        }
        g_cls = (jclass) (*env)->NewGlobalRef(env, k);
        (*env)->DeleteLocalRef(env, k);
        if (g_cls == NULL) {
            ime_log("init: NewGlobalRef(class) failed");
            return 0;
        }
    }
    ctor = (*env)->GetMethodID(env, g_cls, "<init>", "(Landroid/app/Activity;)V");
    if (ctor == NULL) {
        ime_log("init: constructor (Activity) missing");
        log_exception(env);
        return 0;
    }
    obj = (*env)->NewObject(env, g_cls, ctor, g_activity);
    if (obj == NULL) {
        ime_log("init: NewObject(ImeBridge) failed");
        log_exception(env);
        return 0;
    }
    g_bridge = (*env)->NewGlobalRef(env, obj);
    (*env)->DeleteLocalRef(env, obj);
    return g_bridge != NULL ? 1 : 0;
}

/* gg_ime_show 弹出输入法（text 为当前输入框内容，mode 见 Java 侧常量）。 */
void gg_ime_show(const char *text, int mode) {
    JNIEnv *env = env_get();
    jmethodID m;
    jstring s;

    if (env == NULL || g_bridge == NULL) {
        return;
    }
    m = (*env)->GetMethodID(env, g_cls, "show", "(ILjava/lang/String;)V");
    if (m == NULL) {
        log_exception(env);
        return;
    }
    s = (*env)->NewStringUTF(env, text != NULL ? text : "");
    (*env)->CallVoidMethod(env, g_bridge, m, (jint) mode, s);
    (*env)->DeleteLocalRef(env, s);
    log_exception(env);
}

/* gg_ime_hide 收起输入法。 */
void gg_ime_hide(void) {
    JNIEnv *env = env_get();
    jmethodID m;

    if (env == NULL || g_bridge == NULL) {
        return;
    }
    m = (*env)->GetMethodID(env, g_cls, "hide", "()V");
    if (m == NULL) {
        log_exception(env);
        return;
    }
    (*env)->CallVoidMethod(env, g_bridge, m);
    log_exception(env);
}

/* gg_ime_sync 把最新文本同步进隐藏输入框。 */
void gg_ime_sync(const char *text) {
    JNIEnv *env = env_get();
    jmethodID m;
    jstring s;

    if (env == NULL || g_bridge == NULL) {
        return;
    }
    m = (*env)->GetMethodID(env, g_cls, "sync", "(Ljava/lang/String;)V");
    if (m == NULL) {
        log_exception(env);
        return;
    }
    s = (*env)->NewStringUTF(env, text != NULL ? text : "");
    (*env)->CallVoidMethod(env, g_bridge, m, s);
    (*env)->DeleteLocalRef(env, s);
    log_exception(env);
}

/* Java_com_gogui_gallery_GalleryActivity_setBridge
 *
 * Java 侧（NativeActivity 子类）在 onCreate 里把 ImeBridge 交过来。
 * 这是唯一可靠的取对象方式：native 线程没有 context ClassLoader，
 * 自己 FindClass / loadClass 拿不到 dex 里的应用类。 */
JNIEXPORT void JNICALL
Java_com_gogui_gallery_GalleryActivity_setBridge(JNIEnv *env, jobject thiz, jobject bridge) {
    jclass cls;

    (void) thiz;
    if (bridge == NULL) {
        return;
    }
    (*env)->GetJavaVM(env, &g_vm);
    if (g_bridge != NULL) {
        (*env)->DeleteGlobalRef(env, g_bridge);
        g_bridge = NULL;
    }
    g_bridge = (*env)->NewGlobalRef(env, bridge);
    cls = (*env)->GetObjectClass(env, bridge);
    if (cls != NULL) {
        if (g_cls != NULL) {
            (*env)->DeleteGlobalRef(env, g_cls);
        }
        g_cls = (jclass) (*env)->NewGlobalRef(env, cls);
        (*env)->DeleteLocalRef(env, cls);
    }
    __android_log_print(ANDROID_LOG_INFO, "gogui", "ime: bridge attached from Java");
}

/* Java_com_gogui_gallery_ImeBridge_nativeText 输入法上屏回调 → Go。
 *
 * 名字必须与 Java 包名 + 类名完全一致（com.gogui.gallery.ImeBridge 的
 * private static native void nativeText(String)）。 */
JNIEXPORT void JNICALL
Java_com_gogui_gallery_ImeBridge_nativeText(JNIEnv *env, jclass cls, jstring text) {
    const char *utf;

    (void) cls;
    if (text == NULL) {
        return;
    }
    utf = (*env)->GetStringUTFChars(env, text, NULL);
    if (utf == NULL) {
        return;
    }
    ggGoImeText((char *) utf);
    (*env)->ReleaseStringUTFChars(env, text, utf);
}
