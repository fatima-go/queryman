//
// Copyright (c) 2017 SK TECHX.
// All right reserved.
//
// This software is the confidential and proprietary information of SK TECHX.
// You shall not disclose such Confidential Information and
// shall use it only in accordance with the terms of the license agreement
// you entered into with SK TECHX.
//
//
// @project queryman
// @author 1100282
// @date 2017. 10. 26. AM 7:51
//

package queryman

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"strings"
	"testing"
)

var testData = []byte(`
<query>
    <select id="selectDual">
		SELECT 1 FROM <![CDATA[dual]]>
    </select>
    <select id="selectWhere">
		SELECT 1 FROM city
		WHERE a={varA}
		<if key="VarB">
			AND b={varB}
		</if>
		<if key="VarK" exist="false">
			AND k={varK}
		</if>
		<if key="VarB">
			AND b={varB}
		</if>
		AND c={varC}
    </select>
</query>
`)

// go test -v -db=local -user=local -password=angel -host=127.0.0.1:3306
func TestLoaderSimple(t *testing.T) {
	queryNormalizer = newNormalizer("mysql")

	stmtList = make([]QueryStatement, 0)
	buf := bytes.NewBuffer(testData)
	dec := xml.NewDecoder(buf)

	for {
		t, tokenErr := dec.Token()
		if tokenErr != nil {
			if tokenErr == io.EOF {
				break
			}
			panic(tokenErr)
		}

		switch t := t.(type) {
		case xml.StartElement:
			currentId = getAttr(t.Attr, attrId)
			//fmt.Printf("start [%s] : %s\n", currentId, t.Name.Local)
			currentEleType = buildElementType(t.Name.Local)
			if currentEleType.IsSql() {
				currentStmt = newQueryStatement(currentEleType)
				traverseIf(dec)
			}
		case xml.CharData:
			if len(currentId) == 0 {
				break
			}
			currentStmt.Query = currentStmt.Query + string(t)
		case xml.EndElement:
			if currentEleType.IsSql() {
				currentStmt.Query = strings.Trim(currentStmt.Query, cutset)
				//fmt.Printf("end [%s] : %s\n", currentId, currentStmt)
				currentId = ""
			}
		}
	}

	if len(stmtList) != 2 {
		t.Errorf("expect stmt len 2")
	}

	teststmt := stmtList[0]
	if teststmt.eleType != eleTypeSelect {
		t.Fatalf("expect first element type SELECT but %s", teststmt.eleType)
	}
	if teststmt.Id != "selectDual" {
		t.Fatalf("invalid first stmt id : %s", teststmt.Id)
	}
	if teststmt.Query != "SELECT 1 FROM dual" {
		t.Fatalf("invalid first stmt query")
	}

	teststmt = stmtList[1]
	if teststmt.eleType != eleTypeSelect {
		t.Fatalf("expect second element type SELECT but %s", teststmt.eleType)
	}
	if teststmt.Id != "selectWhere" {
		t.Fatalf("invalid second stmt id : %s", teststmt.Id)
	}
	expect := fmt.Sprintf(string(expect1), ifClauseWrappingKey, ifClauseWrappingKey, ifClauseWrappingKey, ifClauseWrappingKey, ifClauseWrappingKey, ifClauseWrappingKey)
	if teststmt.Query != expect {
		t.Fatalf("invalid second stmt query : expect=[%s], query=[%s]", expect, teststmt.Query)
	}
	if len(teststmt.clause) != 3 {
		t.Fatalf("invalid second stmt if cluase list")
	}

	m := make(map[string]interface{})

	refinedStmt, err := teststmt.RefineStatement(m)
	if err != nil {
		t.Fatalf("fail to refine : %s", err.Error())
	}
	if len(strings.Split(refinedStmt.Query, "?")) != 4 {
		t.Fatalf("invalid refined query")
	}

	m["VarB"] = struct{}{}
	refinedStmt, err = teststmt.RefineStatement(m)
	if err != nil {
		t.Fatalf("fail to refine : %s", err.Error())
	}

	if len(strings.Split(refinedStmt.Query, "?")) != 6 {
		t.Fatalf("invalid refined query")
	}

	m["VarK"] = struct{}{}
	refinedStmt, err = teststmt.RefineStatement(m)
	if err != nil {
		t.Fatalf("fail to refine : %s", err.Error())
	}
	if len(strings.Split(refinedStmt.Query, "?")) != 5 {
		t.Fatalf("invalid refined query")
	}
}

var expect1 = []byte(`SELECT 1 FROM city
		WHERE a={varA}
		 %s0%s
		 %s1%s
		 %s2%s
		AND c={varC}`)

func TestLoaderComplicated(t *testing.T) {
	queryNormalizer = newNormalizer("mysql")

	stmtList = make([]QueryStatement, 0)
	buf := bytes.NewBuffer(testData2)
	dec := xml.NewDecoder(buf)

	for {
		t, tokenErr := dec.Token()
		if tokenErr != nil {
			if tokenErr == io.EOF {
				break
			}
			panic(tokenErr)
		}

		switch t := t.(type) {
		case xml.StartElement:
			currentId = getAttr(t.Attr, attrId)
			//fmt.Printf("start [%s] : %s\n", currentId, t.Name.Local)
			currentEleType = buildElementType(t.Name.Local)
			if currentEleType.IsSql() {
				currentStmt = newQueryStatement(currentEleType)
				traverseIf(dec)
			}
		case xml.CharData:
			if len(currentId) == 0 {
				break
			}
			currentStmt.Query = currentStmt.Query + string(t)
		case xml.EndElement:
			if currentEleType.IsSql() {
				currentStmt.Query = strings.Trim(currentStmt.Query, cutset)
				//fmt.Printf("end [%s] : %s\n", currentId, currentStmt)
				currentId = ""
			}
		}
	}

	if len(stmtList) != 43 {
		t.Errorf("expect stmt len 43. %d", len(stmtList))
	}

	//fmt.Printf("total %d stmt list\n", len(stmtList))
	//for i, v := range stmtList {
	//	fmt.Printf("[%d] stmt : %s\n", i, v)
	//}
}

var testData2 = []byte(`
<?xml version="1.0" encoding="UTF-8" ?>
<query>
    <update id="ClearTedTrackTemp">
        TRUNCATE ted_track_temp
    </update>

    <insert id="InsertTedTrackTemp">
        INSERT INTO ted_track_temp(
            track_id,
            disc_id,
            album_id,
            track_no,
            track_title,
            len,
            title_yn,
            download_yn,
            streaming_premium_yn,
            download_premium_yn,
            pps_yn,
            ppd_yn,
            price,
            svc_192_yn,
            svc_320_yn,
            agency_id,
            db_sts,
            row_no
        ) VALUES(
            {TrackId},
            {DiscId},
            {AlbumId},
            {TrackNo},
            {TrackTitle},
            {Len},
            {TitleYn},
            {DownloadYn},
            {StreamingPremiumYn},
            {DownloadPremiumYn},
            {PpsYn},
            {ppdYn},
            {Price},
            {Svc192Yn},
            {Svc320Yn},
            {AgencyId},
            {DbSts},
            {RowNo}
        )
    </insert>

    <insert id="InsertTedAdult">
        INSERT INTO ted_adult (track_id)
            SELECT ? FROM DUAL
        WHERE NOT EXISTS
            (SELECT track_id FROM ted_adult WHERE track_id=?)
    </insert>
    <insert id="DeleteTedAdult">
        DELETE FROM ted_adult WHERE track_id = {trackId}
    </insert>
    <insert id="InsertTedAgency">
        INSERT INTO ted_agency (agency_id, agency_nm)
        SELECT ?, ? FROM DUAL
        WHERE NOT EXISTS
        (SELECT agency_id, agency_nm FROM ted_agency WHERE agency_id=?)
    </insert>
    <insert id="InsertTedAlbum">
        INSERT INTO ted_album
        (
            album_id,
            title,
            release_ymd,
            disc_cnt,
            nation_cd,
            edition_no,
            album_tp,
            album_buy_yn,
            album_buy_amt,
            db_sts
        )
        VALUES
        (
            {AlbumId},
            {Title},
            {ReleaseYmd},
            {DiscCnt},
            {NationCd},
            {EditionNo},
            {AlbumTp},
            {AlbumBuyYn},
            {AlbumBuyAmt},
            {DbSts}
        )
        ON DUPLICATE KEY
        UPDATE
            title = {Title},
            release_ymd = {ReleaseYmd},
            disc_cnt = {DiscCnt},
            nation_cd = {NationCd},
            edition_no = {EditionNo},
            album_tp = {AlbumTp},
            album_buy_yn = {AlbumBuyYn},
            album_buy_amt = {AlbumBuyAmt},
            db_sts = "A"
    </insert>
    <insert id="InsertTedAlbumArtist">
        INSERT INTO ted_albumartist
        (
            albumartist_id,
            artist_id,
            album_id,
            rp_yn,
            listorder,
            db_sts
        )
        VALUES
        (
            {AlbumArtistId},
            {ArtistId},
            {AlbumId},
            {RpYn},
            {Listorder},
            {DbSts}
        )
        ON DUPLICATE KEY
        UPDATE
            db_sts = "A",
            rp_yn = {RpYn},
            listorder = {Listorder}
    </insert>
    <insert id="InsertTedAlbumStyle">
        INSERT INTO ted_albumstyle
        (
            album_id,
            style_id,
            db_sts
        )
        VALUES
        (
            {AlbumId},
            {StyleId},
            {DbSts}
        )
        ON DUPLICATE KEY
        UPDATE
            style_id = {StyleId},
            db_sts = "A"
    </insert>
    <insert id="InsertTedAlbumImage">
        INSERT INTO ted_albumimage(
            album_id,
            size,
            url
        ) VALUES (
            {AlbumId},
            {Size},
            {Url}
        )
        ON DUPLICATE KEY
        UPDATE
            url = {Url}
    </insert>
    <insert id="InsertTbAlbumImage">
        INSERT INTO tb_album_img (
            album_id,
            chnl_type,
            album_img_size,
            album_img_url,
            create_dtime,
            update_dtime
        ) VALUES (
            {AlbumId},
            "MM",
            {Size},
            {Url},
            now(),
            now()
        )
        ON DUPLICATE KEY
        UPDATE
            album_img_url = {Url},
            update_dtime = now()
    </insert>
    <insert id="InsertTedArtist">
        INSERT INTO ted_artist(
            artist_id,
            artist_nm,
            grp_cd,
            sex_cd,
            act_start_ymd,
            act_end_ymd,
            nation_cd,
            db_sts
        ) VALUES (
            {ArtistId},
            {ArtistNm},
            {GrpCd},
            {SexCd},
            {ActStartYmd},
            {ActEndYmd},
            {NationCd},
            {DbSts}
        )
        ON DUPLICATE KEY
        UPDATE
            artist_nm = {ArtistNm},
            grp_cd = {GrpCd},
            sex_cd = {SexCd},
            act_start_ymd = {ActStartYmd},
            act_end_ymd = {ActEndYmd},
            nation_cd = {NationCd},
            db_sts = "A"
    </insert>
    <insert id="InsertTedArtistGroup">
        INSERT INTO ted_artistgroup (
            group_id,
            member_id,
            act_yn,
            db_sts
        ) VALUES(
            {GroupId},
            {MemberId},
            {ActYn},
            {DbSts}
        )
        ON DUPLICATE KEY
        UPDATE
            act_yn = {ActYn},
            db_sts = "A"
    </insert>
    <insert id="InsertTedArtistImage">
        INSERT INTO ted_artistimage(
            artist_id,
            size,
            url
        ) VALUES (
            {ArtistId},
            {Size},
            {Url}
        )
        ON DUPLICATE KEY
        UPDATE
            url = {Url}
    </insert>
    <insert id="InsertTedArtistStyle">
        INSERT INTO ted_artiststyle
        (
            artist_id,
            style_id,
            db_sts
        )
        VALUES
        (
            {ArtistId},
            {StyleId},
            {DbSts}
        )
        ON DUPLICATE KEY
        UPDATE
            db_sts = "A"
    </insert>
    <insert id="InsertTedCode">
        INSERT INTO ted_code
        (
            cd_id,
            cd_nm,
            db_sts
        )
        VALUES
        (
            {CodeId},
            {CodeName},
            {DbSts}
        )
        ON DUPLICATE KEY
        UPDATE
            cd_nm = {CodeName},
            db_sts = "A"
    </insert>
    <insert id="InsertTedCodeDtl">
        INSERT INTO ted_codedtl
        (
            cd_dtl_cd,
            cd_id,
            cd_dtl_nm,
            db_sts
        )
        VALUES
        (
            {CodeDtlCode},
            {CodeId},
            {CodeDtlName},
            {DbSts}
        )
        ON DUPLICATE KEY
        UPDATE
            cd_dtl_nm = {CodeDtlName},
            db_sts = "A"
    </insert>
    <insert id="InsertTedDisc">
        INSERT INTO ted_disk
        (
            album_id,
            disc_id,
            disc_no,
            db_sts
        )
        VALUES
        (
            {AlbumId},
            {DiscId},
            {DiscNo},
            {DbSts}
        )
        ON DUPLICATE KEY
        UPDATE
            disc_id = {DiscId},
            disc_no = {DiscNo},
            db_sts = "A"
    </insert>
    <insert id="InsertTedStyle">
        INSERT INTO ted_style
        (
            genre_id,
            style_id,
            style_nm,
            db_sts
        )
        VALUES
        (
            {GenreId},
            {StyleId},
            {StyleName},
            {DbSts}
        )
        ON DUPLICATE KEY
        UPDATE
            style_nm = {StyleName},
            db_sts = "A"
    </insert>
    <insert id="InsertTedTrackArtist">
        INSERT INTO ted_trackartist
        (
            trackartist_id,
            artist_id,
            track_id,
            rp_yn,
            listorder,
            role_cd,
            db_sts
        )
        VALUES
        (
            {TrackArtistId},
            {ArtistId},
            {TrackId},
            {RpYn},
            {Listorder},
            {RoleCd},
            {DbSts}
        )
        ON DUPLICATE KEY
        UPDATE
            artist_id = {ArtistId},
            track_id = {TrackId},
            rp_yn = {RpYn},
            listorder = {Listorder},
            role_cd = {RoleCd},
            db_sts = "A"
    </insert>
    <insert id="InsertTedTrack">
        INSERT INTO ted_track (
            track_id,
            disc_id,
            album_id,
            track_no,
            track_title,
            len,
            title_yn,
            download_yn,
            streaming_premium_yn,
            download_premium_yn,
            pps_yn,
            ppd_yn,
            price,
            svc_192_yn,
            svc_320_yn,
            agency_id,
            db_sts
        ) VALUES(
            {TrackId},
            {DiscId},
            {AlbumId},
            {TrackNo},
            {TrackTitle},
            {Len},
            {TitleYn},
            {DownloadYn},
            {StreamingPremiumYn},
            {DownloadPremiumYn},
            {PpsYn},
            {PpdYn},
            {Price},
            {Svc192Yn},
            {Svc320Yn},
            {AgencyId},
            {DbSts}
        )
        ON DUPLICATE KEY
        UPDATE
            disc_id = {DiscId},
            album_id = {AlbumId},
            track_no = {TrackNo},
            track_title = {TrackTitle},
            len = {Len},
            title_yn = {TitleYn},
            download_yn = {DownloadYn},
            streaming_premium_yn = {StreamingPremiumYn},
            download_premium_yn = {DownloadPremiumYn},
            pps_yn = {PpsYn},
            ppd_yn = {PpdYn},
            price = {Price},
            svc_192_yn = {Svc192Yn},
            svc_320_yn = {Svc320Yn},
            agency_id = {AgencyId},
            db_sts = "A"
    </insert>
    <delete id="DeleteTedAlbum">
        DELETE FROM ted_album WHERE album_id = {AlbumId}
    </delete>
    <delete id="DeleteTedAlbumArtist">
        DELETE FROM ted_albumartist WHERE albumartist_id = {AlbumArtistId}
    </delete>
    <delete id="DeleteTedAlbumStyle">
        DELETE FROM ted_albumstyle WHERE album_id = {AlbumId}
    </delete>
    <delete id="DeleteTedArtist">
        DELETE FROM ted_artist WHERE artist_id = {ArtistId}
    </delete>
    <delete id="DeleteTedArtistGroup">
        DELETE FROM ted_artistgroup WHERE group_id = {GroupId}
    </delete>
    <delete id="DeleteTedArtistStyle">
        DELETE FROM ted_albumstyle WHERE album_id = {ArtistId}
    </delete>
    <delete id="DeleteTedCode">
        DELETE FROM ted_code WHERE cd_id = {CodeId}
    </delete>
    <delete id="DeleteTedCodeDtl">
        DELETE FROM ted_codedtl WHERE cd_dtl_cd = {CodeDtlCode}
    </delete>
    <delete id="DeleteTedDisc">
        DELETE FROM ted_disk WHERE album_id = {AlbumId}
    </delete>
    <delete id="DeleteTedStyle">
        DELETE FROM ted_style WHERE genre_id = {GenreId} AND style_id = {StyleId}
    </delete>
    <delete id="DeleteTedTrackArtist">
        DELETE FROM ted_trackartist WHERE trackartist_id = {TrackArtistId}
    </delete>
    <delete id="DeleteTedTrack">
        DELETE FROM ted_track WHERE track_id = {TrackId}
    </delete>

    <select id="SelectTedTrackTemp">
        SELECT
            track_id,
            disc_id,
            album_id,
            track_no,
            track_title,
            len,
            title_yn,
            download_yn,
            streaming_premium_yn,
            download_premium_yn,
            pps_yn,
            ppd_yn,
            price,
            svc_192_yn,
            svc_320_yn,
            agency_id,
            db_sts
        FROM ted_track_temp ORDER BY row_no ASC
    </select>
    <select id="SelectTedTrackList">
        SELECT
            a.track_id				AS track_id,
            a.disc_id               AS disc_id,
            a.track_title			AS track_nm,
            a.album_id				AS album_id,
            a.track_no              AS track_no,
            a.len					AS track_play_tm,
            CASE
            <![CDATA[
            WHEN b.track_id is not null  THEN 'Y'  ELSE 'N'
            ]]>
            END 					AS adult_auth_need_track_yn,
            a.streaming_premium_yn	AS streaming_premium_yn,
            a.pps_yn				AS pps_yn,
            'Y'						AS disp_status_yn,
            1						AS track_subtract_qty,
            0						AS track_popularity,
            now()					AS create_dtime,
            now()					AS update_dtime,
            CASE
            WHEN a.title_yn = NULL THEN 'N'
            WHEN a.title_yn = '' THEN 'N'
            ELSE a.title_yn
            END 					AS title_yn,
            a.agency_id             AS agency_id,
            a.db_sts				AS db_sts
        FROM ted_track_temp A LEFT JOIN ted_adult B
        ON A.track_id = B.track_id
    </select>


    <update id="UpdateTbTrack">
        UPDATE tb_track
        SET
            disp_status_yn = 'N',
            update_dtime = now()
        WHERE
            track_id = {trackId}
    </update>

    <select id="SelectChnlIdFromMap">
        SELECT DISTINCT(chnl_id) FROM tb_map_chnl_track WHERE track_id IN ({TrackIdList})
    </select>

    <update id="UpdateTbTrackCount">
        <![CDATA[
update tb_chnl c , (
        select
            b.chnl_id chnl_id         ,
            sum( b.track_cnt ) track_cnt         ,
            case
                when length( cast( sum( b.track_tm_sec ) /60 as unsigned )  ) <= 2             then lpad( cast( sum( b.track_tm_sec ) /60 as unsigned )  ,
                2,
                '0')
                else cast( sum( b.track_tm_sec ) /60 as unsigned )
            end as track_tm_min
        from
            (         select
                m.chnl_id                 ,
                1 track_cnt                 ,
                substr(track_play_tm ,
                1,
                2 ) * 60 + substr(track_play_tm,
                4,
                2 ) track_tm_sec
            from
                tb_track t
            join
                tb_map_chnl_track m
                    on t.track_id = m.track_id
            where
                t.disp_status_yn = 'Y' AND m.chnl_id={ChnlId}    ) b
        group by
            b.chnl_id ) d
        set
            c.track_cnt = d.track_cnt     ,
            c.chnl_play_tm = d.track_tm_min
        where
            c.chnl_id = d.chnl_id AND c.chnl_id={ChnlId}
]]>
    </update>


    <select id="SelectLyricsTrackExist">
        SELECT count(track_id)
        FROM tb_track
        WHERE track_id = {TrackId}
    </select>

    <delete id="DeleteTedLyrics">
        DELETE FROM ted_lyrics WHERE track_id = {TrackId}
    </delete>

    <insert id="InsertTedLyrics">
        INSERT INTO ted_lyrics (
            track_id, lyrics_tp, lyrics, db_sts
        ) VALUES (
            {TrackId}, {LyricsTp}, {Lyrics}, {DbSts}
        )
        ON DUPLICATE KEY
        UPDATE
            db_sts = "A",
            lyrics_tp = {LyricsTp},
            lyrics = {Lyrics}
    </insert>

    <insert id="InsertTbTrack">
        INSERT INTO tb_track (
            track_id,
            track_nm,
            album_id,
            disk_id,
            track_no,
            track_play_tm,
            adult_auth_need_track_yn,
            streaming_premium_yn,
            pps_yn,
            disp_status_yn,
            agency_id,
            track_subtrct_qty,
            track_popularity,
            create_dtime,
            update_dtime,
            title_yn
        ) VALUES (
            {trackId},
            {trackName},
            {albumId},
            {diskId},
            {trackNo},
            {trackPlayTime},
            {adultAuthNeedTrackYn},
            {streamingPremiumYn},
            {ppsYn},
            {displayStatusYn},
            {agencyId},
            {trackSubtractQuantity},
            {trackPopularity},
            now(),
            now(),
            {titleYn}
        )
        ON DUPLICATE KEY UPDATE
            track_nm = {trackName},
            album_id = {albumId},
            disk_id = {diskId},
            track_no = {trackNo},
            track_play_tm = {trackPlayTime},
            adult_auth_need_track_yn = {adultAuthNeedTrackYn},
            streaming_premium_yn = {streamingPremiumYn},
            pps_yn = {ppsYn},
            disp_status_yn = {displayStatusYn},
            agency_id = {agencyId},
            track_subtrct_qty = {trackSubtractQuantity},
            track_popularity = {trackPopularity},
            title_yn = {titleYn},
            update_dtime = now()
    </insert>

    <select id="SelectChnlIdWithTrackId">
        SELECT DISTINCT(chnl_id) FROM tb_map_chnl_track WHERE track_id={TrackId}
    </select>

    <update id="UpdateChnlImage">
        UPDATE    tb_chnl a
            JOIN tb_map_chnl_track b ON b.chnl_id = a.chnl_id
            JOIN (
                SELECT mt.chnl_id, min(track_sn) track_sn
                FROM tb_map_chnl_track mt
                JOIN tb_track tt ON tt.track_id = mt.track_id
                WHERE tt.disp_status_yn = 'Y' AND mt.chnl_id = {ChnlId}
                GROUP BY mt.chnl_id ) f ON b.chnl_id = f.chnl_id AND b.track_sn = f.track_sn
        JOIN tb_track t ON t.track_id = b.track_id
        SET a.album_id = t.album_id
        WHERE t.disp_status_yn = 'Y' AND a.chnl_id={ChnlId}
    </update>
</query>
`)
