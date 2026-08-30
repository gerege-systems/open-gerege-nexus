# docs.nexus.gerege.mn — MkDocs

Энэ хавтас нь [docs.nexus.gerege.mn](https://docs.nexus.gerege.mn/) сайтыг
угсарна. Хэрэгсэл нь [docs.gerege.mn](https://docs.gerege.mn/)-тэй ижил:
**MkDocs + Material for MkDocs**, ижил брэндийн өнгө. Хоёр сайт нь нэг
экосистемийн баримт байх ёстой болохоос, нэг компанийг хуваалцдаг хоёр өөр
бүтээгдэхүүн шиг уншигдах ёсгүй.

## Юу нийтлэгдэхийг хаанаас шийддэг вэ

`../site/pages.mjs` доторх `PAGES` жагсаалт — **энд биш**. Тэр файл нь юуг
нийтлэх, юу гэж нэрлэх, аль бүлэгт хамаарахыг аль хэдийн шийддэг. Хоёр
жагсаалт байвал салах бөгөөд салсан нь нь хэн ч харахгүй байгаа нь болно.

Шинэ баримт нэмэхэд `pages.mjs`-д нэг мөр нэмнэ; энэ хавтсанд юу ч хийхгүй.

## Ажиллуулах

```bash
cd docs/mkdocs
sh build.sh          # угсрана → docs/mkdocs/build/site
```

`build.sh` нь `stage.mjs`-ээр модыг бэлдээд MkDocs-ыг **контейнерээр**
ажиллуулна: угсралт хаана ч ижил үр дүн өгөх ёстой, мөн энэ репод Python-ий
хамаарал нэмэхгүй.

macOS дээр Docker Desktop нь `/private/tmp` доорх замыг хуваалцдаггүй —
репо `/Users` дор байвал асуудалгүй.

## Байршуулах

```bash
sh deploy.sh nexus-root      # угсраад /var/www/docs руу хуулна
```

nginx-ийн тохиргоо нь `deploy/nginx/docs.nexus.gerege.mn.conf`.

## Яагаад `docs/site` хэвээр байгаа вэ

`docs/site` нь GitHub Pages
([gerege-systems.github.io/open-gerege-nexus](https://gerege-systems.github.io/open-gerege-nexus/))
дээрх сайтыг угсардаг өөр builder. Хоёулаа нэг `pages.mjs`-ээс уншдаг тул
агуулга нь салахгүй, гэхдээ хоёр угсрагч байх нь урт хугацаанд хадгалах зүйл
биш. Аль нэгийг нь тэтгэвэрт гаргах шийдвэрийг тусад нь гаргана.
