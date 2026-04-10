declare namespace App {
  /**
   * I18n namespace
   *
   * Locales type
   */
  namespace I18n {
    type RouteKey = string;

    type LangType = 'en-US' | 'zh-CN';

    type LangOption = {
      label: string;
      key: LangType;
    };

    type I18nRouteKey = Exclude<RouteKey, 'root' | 'not-found'>;

    type FormMsg = {
      required: string;
      invalid: string;
    };

    type Schema = {
      system: {
        title: string;
        updateTitle: string;
        updateContent: string;
        updateConfirm: string;
        updateCancel: string;
      };
      common: {
        action: string;
        add: string;
        addSuccess: string;
        backToHome: string;
        batchDelete: string;
        cancel: string;
        close: string;
        check: string;
        expandColumn: string;
        columnSetting: string;
        config: string;
        confirm: string;
        delete: string;
        deleteSuccess: string;
        confirmDelete: string;
        edit: string;
        warning: string;
        error: string;
        index: string;
        keywordSearch: string;
        logout: string;
        logoutConfirm: string;
        lookForward: string;
        modify: string;
        modifySuccess: string;
        noData: string;
        operate: string;
        pleaseCheckValue: string;
        pleaseInput: string;
        pleaseSelect: string;
        enable: string;
        disable: string;
        yes: string;
        no: string;
        refresh: string;
        reset: string;
        search: string;
        status: string;
        switch: string;
        tip: string;
        trigger: string;
        update: string;
        updateSuccess: string;
        updateFailed: string;
        upload: string;
        select: string;
        adminProfile: string;
        save: string;
        createdAt: string;
        yesOrNo: {
          yes: string;
          no: string;
        };
      };
      request: {
        logout: string;
        logoutMsg: string;
        logoutWithModal: string;
        logoutWithModalMsg: string;
        refreshToken: string;
        tokenExpired: string;
        backend: {
          unknown: string;
          invalidParams: string;
          unauthorized: string;
          forbidden: string;
          notFound: string;
          internalError: string;
          tooManyRequest: string;
          badRequest: string;
          alreadyExists: string;
          userNotFound: string;
          userDisabled: string;
          passwordWrong: string;
          userAlreadyExists: string;
          tokenExpired: string;
          tokenInvalid: string;
          oldPasswordWrong: string;
          roleNotFound: string;
          roleInUse: string;
          roleAlreadyExists: string;
          roleCodeDuplicate: string;
          cannotDeleteSuper: string;
          cannotModifySuper: string;
          menuNotFound: string;
          menuHasChildren: string;
          menuAlreadyExists: string;
          menuRouteDuplicate: string;
          buttonNotFound: string;
          buttonAlreadyExists: string;
          buttonCodeDuplicate: string;
          apiNotFound: string;
          apiAlreadyExists: string;
          apiPathDuplicate: string;
        };
      };
      theme: {
        themeSchema: { title: string } & Record<UnionKey.ThemeScheme, string>;
        grayscale: string;
        colourWeakness: string;
        layoutMode: { title: string; reverseHorizontalMix: string } & Record<UnionKey.ThemeLayoutMode, string>;
        recommendColor: string;
        recommendColorDesc: string;
        themeColor: {
          title: string;
          followPrimary: string;
        } & Theme.ThemeColor;
        scrollMode: { title: string } & Record<UnionKey.ThemeScrollMode, string>;
        page: {
          animate: string;
          mode: { title: string } & Record<UnionKey.ThemePageAnimateMode, string>;
        };
        fixedHeaderAndTab: string;
        header: {
          height: string;
          breadcrumb: {
            visible: string;
            showIcon: string;
          };
          multilingual: {
            visible: string;
          };
          globalSearch: {
            visible: string;
          };
        };
        tab: {
          visible: string;
          cache: string;
          height: string;
          mode: { title: string } & Record<UnionKey.ThemeTabMode, string>;
        };
        sider: {
          inverted: string;
          width: string;
          collapsedWidth: string;
          mixWidth: string;
          mixCollapsedWidth: string;
          mixChildMenuWidth: string;
        };
        footer: {
          visible: string;
          fixed: string;
          height: string;
          right: string;
        };
        watermark: {
          visible: string;
          text: string;
          enableUserName: string;
        };
        themeDrawerTitle: string;
        pageFunTitle: string;
        resetCacheStrategy: { title: string } & Record<UnionKey.ResetCacheStrategy, string>;
        configOperation: {
          copyConfig: string;
          copySuccessMsg: string;
          resetConfig: string;
          resetSuccessMsg: string;
        };
      };
      route: Record<I18nRouteKey, string>;
      page: {
        login: {
          common: {
            loginOrRegister: string;
            userNamePlaceholder: string;
            phonePlaceholder: string;
            codePlaceholder: string;
            passwordPlaceholder: string;
            confirmPasswordPlaceholder: string;
            confirm: string;
            back: string;
            validateSuccess: string;
            loginSuccess: string;
            welcomeBack: string;
          };
          pwdLogin: {
            title: string;
          };
        };
        home: {
          branchDesc: string;
          greeting: string;
          weatherDesc: string;
          projectCount: string;
          todo: string;
          message: string;
          downloadCount: string;
          registerCount: string;
          schedule: string;
          study: string;
          work: string;
          rest: string;
          entertainment: string;
          visitCount: string;
          turnover: string;
          dealCount: string;
          projectNews: {
            title: string;
            moreNews: string;
            desc1: string;
            desc2: string;
            desc3: string;
            desc4: string;
            desc5: string;
          };
          creativity: string;
        };
        function: {
          tab: {
            tabOperate: {
              title: string;
              addTab: string;
              addTabDesc: string;
              closeTab: string;
              closeCurrentTab: string;
              closeAboutTab: string;
              addMultiTab: string;
              addMultiTabDesc1: string;
              addMultiTabDesc2: string;
            };
            tabTitle: {
              title: string;
              changeTitle: string;
              change: string;
              resetTitle: string;
              reset: string;
            };
          };
          multiTab: {
            routeParam: string;
            backTab: string;
          };
          toggleAuth: {
            toggleAccount: string;
            authHook: string;
            superAdminVisible: string;
            adminVisible: string;
            adminOrUserVisible: string;
          };
          request: {
            repeatedErrorOccurOnce: string;
            repeatedError: string;
            repeatedErrorMsg1: string;
            repeatedErrorMsg2: string;
          };
        };
        manage: {
          common: {
            status: {
              enable: string;
              disable: string;
            };
          };
          role: {
            title: string;
            roleName: string;
            roleCode: string;
            roleStatus: string;
            roleDesc: string;
            form: {
              roleName: string;
              roleCode: string;
              roleStatus: string;
              roleDesc: string;
            };
            addRole: string;
            editRole: string;
            menuAuth: string;
            buttonAuth: string;
            apiAuth: string;
          };
          admin: {
            title: string;
            userName: string;
            userGender: string;
            nickName: string;
            userPhone: string;
            userEmail: string;
            userStatus: string;
            userRole: string;
            form: {
              userName: string;
              userGender: string;
              nickName: string;
              userPhone: string;
              userEmail: string;
              userStatus: string;
              userRole: string;
            };
            addAdmin: string;
            editAdmin: string;
            gender: {
              male: string;
              female: string;
            };
          };
          menu: {
            home: string;
            title: string;
            id: string;
            parentId: string;
            menuType: string;
            menuName: string;
            routeName: string;
            routePath: string;
            pathParam: string;
            layout: string;
            page: string;
            i18nKey: string;
            icon: string;
            localIcon: string;
            iconTypeTitle: string;
            order: string;
            constant: string;
            keepAlive: string;
            href: string;
            hideInMenu: string;
            activeMenu: string;
            multiTab: string;
            fixedIndexInTab: string;
            query: string;
            button: string;
            buttonCode: string;
            buttonDesc: string;
            menuStatus: string;
            form: {
              home: string;
              menuType: string;
              menuName: string;
              routeName: string;
              routePath: string;
              pathParam: string;
              layout: string;
              page: string;
              i18nKey: string;
              icon: string;
              localIcon: string;
              order: string;
              keepAlive: string;
              href: string;
              hideInMenu: string;
              activeMenu: string;
              multiTab: string;
              fixedInTab: string;
              fixedIndexInTab: string;
              queryKey: string;
              queryValue: string;
              button: string;
              buttonCode: string;
              buttonDesc: string;
              menuStatus: string;
            };
            addMenu: string;
            editMenu: string;
            addChildMenu: string;
            type: {
              directory: string;
              menu: string;
              button: string;
            };
            iconType: {
              iconify: string;
              local: string;
            };
          };
          storage: {
            configTitle: string;
            configName: string;
            provider: string;
            endpoint: string;
            region: string;
            bucket: string;
            accessKey: string;
            secretKey: string;
            secretKeyPlaceholder: string;
            domain: string;
            pathPrefix: string;
            isDefault: string;
            status: string;
            maxFileSize: string;
            allowedTypes: string;
            allowedTypesPlaceholder: string;
            stsExpireTime: string;
            remark: string;
            setDefault: string;
            addConfig: string;
            editConfig: string;
          };
          upload: {
            title: string;
            preview: string;
            fileName: string;
            fileSize: string;
            source: string;
            businessType: string;
            storageName: string;
            uploaderIp: string;
            uploadedAt: string;
            view: string;
          };
        };
        content: {
          category: {
            title: string;
            categoryName: string;
            parentId: string;
            icon: string;
            iconType: string;
            sort: string;
            status: string;
            contentType: string;
            contentTypePlain: string;
            contentTypeRich: string;
            description: string;
            form: {
              categoryName: string;
              parentId: string;
              icon: string;
              iconType: string;
              sort: string;
              status: string;
              contentType: string;
              description: string;
            };
            addCategory: string;
            editCategory: string;
          };
          article: {
            title: string;
            titleField: string;
            categoryId: string;
            cover: string;
            summary: string;
            content: string;
            author: string;
            source: string;
            keywords: string;
            tags: string;
            viewCount: string;
            likeCount: string;
            isTop: string;
            isHot: string;
            isRecommend: string;
            allowComment: string;
            topOrder: string;
            status: string;
            publishedAt: string;
            statusDraft: string;
            statusPublished: string;
            statusUnpublished: string;
            form: {
              title: string;
              categoryId: string;
              cover: string;
              summary: string;
              content: string;
              author: string;
              source: string;
              keywords: string;
              tags: string;
              isTop: string;
              isHot: string;
              isRecommend: string;
              allowComment: string;
              topOrder: string;
              status: string;
            };
            addArticle: string;
            editArticle: string;
            publish: string;
            unpublish: string;
            setTop: string;
          };
          bannerGroup: {
            title: string;
            groupName: string;
            groupCode: string;
            description: string;
            status: string;
            form: {
              groupName: string;
              groupCode: string;
              description: string;
              status: string;
            };
            addGroup: string;
            editGroup: string;
          };
          bannerItem: {
            title: string;
            groupId: string;
            titleField: string;
            image: string;
            link: string;
            sort: string;
            status: string;
            startTime: string;
            endTime: string;
            form: {
              groupId: string;
              title: string;
              image: string;
              link: string;
              sort: string;
              status: string;
              startTime: string;
              endTime: string;
            };
            addItem: string;
            editItem: string;
          };
        };
        adminProfile: {
          welcome: string;
          profileTab: string;
          passwordTab: string;
          oldPassword: string;
          newPassword: string;
          confirmPassword: string;
          changePassword: string;
          passwordRule: string;
          passwordMismatch: string;
          passwordChangeSuccess: string;
          createTime: string;
          form: {
            oldPassword: string;
            newPassword: string;
            confirmPassword: string;
          };
        };
      };
      form: {
        required: string;
        userName: FormMsg;
        phone: FormMsg;
        pwd: FormMsg;
        confirmPwd: FormMsg;
        code: FormMsg;
        email: FormMsg;
      };
      dropdown: Record<Global.DropdownKey, string>;
      icon: {
        themeConfig: string;
        themeSchema: string;
        lang: string;
        fullscreen: string;
        fullscreenExit: string;
        reload: string;
        collapse: string;
        expand: string;
        pin: string;
        unpin: string;
      };
      datatable: {
        itemCount: string;
      };
    };

    type GetI18nKey<T extends Record<string, unknown>, K extends keyof T = keyof T> = K extends string
      ? T[K] extends Record<string, unknown>
        ? `${K}.${GetI18nKey<T[K]>}`
        : K
      : never;

    type I18nKey = GetI18nKey<Schema>;

    type FlexibleI18nKey = I18nKey | (string & {});

    type TranslateOptions<Locales extends string> = import('vue-i18n').TranslateOptions<Locales>;

    interface $T {
      (key: FlexibleI18nKey): string;
      (key: FlexibleI18nKey, plural: number, options?: TranslateOptions<LangType>): string;
      (key: FlexibleI18nKey, defaultMsg: string, options?: TranslateOptions<I18nKey>): string;
      (key: FlexibleI18nKey, list: unknown[], options?: TranslateOptions<I18nKey>): string;
      (key: FlexibleI18nKey, list: unknown[], plural: number): string;
      (key: FlexibleI18nKey, list: unknown[], defaultMsg: string): string;
      (key: FlexibleI18nKey, named: Record<string, unknown>, options?: TranslateOptions<LangType>): string;
      (key: FlexibleI18nKey, named: Record<string, unknown>, plural: number): string;
      (key: FlexibleI18nKey, named: Record<string, unknown>, defaultMsg: string): string;
    }
  }
}
